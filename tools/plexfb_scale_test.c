/*
 * SPDX-License-Identifier: GPL-3.0-or-later
 *
 * Checks the presenter's picture geometry and scaler (arm/plexfb.c) without
 * MiSTer hardware: where each frame shape lands on the 4:3 raster, that the
 * scaler keeps flat colour flat, ramps monotonic and borders black, that a
 * 720x480 frame passes through untouched, and (built for the ARM) that the
 * NEON path gives exactly the bytes of the plain C one.
 *
 *   cc -O2 -Wall -Wno-unused-function -pthread -o plexfb_scale_test tools/plexfb_scale_test.c -lm
 *   ./plexfb_scale_test [time]
 */
#define PLEXFB_TEST
#include "../arm/plexfb.c"

static int checks, fails;
#define CHECK(c, ...) do { checks++; if (!(c)) { fails++; printf("FAIL line %d: ", __LINE__); \
	printf(__VA_ARGS__); printf("\n"); } } while (0)

#define SLOT_BYTES (W * H * 3 / 2)

static void fits(int w, int h, double aspect, int ex, int ey, int ew, int eh)
{
	static struct geometry g;
	geometry_setup(&g, w, h, aspect);
	const struct plane_map *p = &g.p[0];
	CHECK(p->dx == ex && p->dy == ey && p->dw == ew && p->dh == eh,
	      "%dx%d aspect %.4f -> %dx%d at %d,%d, expected %dx%d at %d,%d",
	      w, h, aspect, p->dw, p->dh, p->dx, p->dy, ew, eh, ex, ey);
	const struct plane_map *u = &g.p[1];
	CHECK(u->dx == ex / 2 && u->dy == ey / 2 && u->dw == ew / 2 && u->dh == eh / 2,
	      "%dx%d chroma rectangle %dx%d at %d,%d", w, h, u->dw, u->dh, u->dx, u->dy);
}

static uint8_t *frame_new(int w, int h)
{
	uint8_t *f = malloc(RAM_BYTES);
	memset(f, 0xAA, RAM_BYTES);      /* slack past the frame is garbage on purpose */
	(void)w; (void)h;
	return f;
}

static size_t chroma_bytes(int w, int h) { return (size_t)((w + 1) / 2) * ((h + 1) / 2); }

static void fill_random(uint8_t *f, int w, int h, unsigned seed)
{
	size_t n = (size_t)w * h + 2 * chroma_bytes(w, h);
	for (size_t i = 0; i < n; i++) { seed = seed * 1103515245u + 12345u; f[i] = (uint8_t)(seed >> 16); }
}

/* output plane pointers in a slot */
static uint8_t *out_y(uint8_t *s) { return s; }
static uint8_t *out_u(uint8_t *s) { return s + W * H; }
static uint8_t *out_v(uint8_t *s) { return s + W * H + (W / 2) * (H / 2); }

static void flat_and_borders(int w, int h, double aspect)
{
	static struct geometry g;
	geometry_setup(&g, w, h, aspect);
	uint8_t *f = frame_new(w, h), *slot = malloc(SLOT_BYTES);
	size_t y = (size_t)w * h, c = chroma_bytes(w, h);
	memset(f, 100, y); memset(f + y, 90, c); memset(f + y + c, 160, c);
	scale_frame(&g, f, slot);
	const struct plane_map *p = &g.p[0], *q = &g.p[1];
	int bad_pic = 0, bad_border = 0;
	for (int r = 0; r < H; r++)
		for (int x = 0; x < W; x++) {
			int in = r >= p->dy && r < p->dy + p->dh && x >= p->dx && x < p->dx + p->dw;
			uint8_t v = out_y(slot)[r * W + x];
			if (in ? v != 100 : v != 16) { if (in) bad_pic++; else bad_border++; }
		}
	for (int r = 0; r < H / 2; r++)
		for (int x = 0; x < W / 2; x++) {
			int in = r >= q->dy && r < q->dy + q->dh && x >= q->dx && x < q->dx + q->dw;
			uint8_t u = out_u(slot)[r * (W / 2) + x], v = out_v(slot)[r * (W / 2) + x];
			if (in ? (u != 90 || v != 160) : (u != 128 || v != 128)) { if (in) bad_pic++; else bad_border++; }
		}
	CHECK(!bad_pic && !bad_border, "%dx%d flat frame: %d picture and %d border samples off", w, h, bad_pic, bad_border);
	free(f); free(slot);
}

static void ramps(int w, int h, double aspect)
{
	static struct geometry g;
	geometry_setup(&g, w, h, aspect);
	uint8_t *f = frame_new(w, h), *slot = malloc(SLOT_BYTES);
	size_t y = (size_t)w * h, c = chroma_bytes(w, h);
	for (int r = 0; r < h; r++)                 /* luma ramps left to right, chroma top to bottom */
		for (int x = 0; x < w; x++) f[(size_t)r * w + x] = (uint8_t)(16 + x * 219 / (w - 1));
	int cw = (w + 1) / 2, ch = (h + 1) / 2;
	for (int r = 0; r < ch; r++)
		for (int x = 0; x < cw; x++) f[y + (size_t)r * cw + x] = f[y + c + (size_t)r * cw + x] = (uint8_t)(16 + r * 224 / (ch - 1));
	scale_frame(&g, f, slot);
	const struct plane_map *p = &g.p[0], *q = &g.p[1];
	int back = 0;
	for (int r = p->dy; r < p->dy + p->dh; r++)
		for (int x = p->dx + 1; x < p->dx + p->dw; x++)
			if (out_y(slot)[r * W + x] < out_y(slot)[r * W + x - 1]) back++;
	for (int x = q->dx; x < q->dx + q->dw; x++)
		for (int r = q->dy + 1; r < q->dy + q->dh; r++)
			if (out_u(slot)[r * (W / 2) + x] < out_u(slot)[(r - 1) * (W / 2) + x]) back++;
	uint8_t first = out_y(slot)[p->dy * W + p->dx], last = out_y(slot)[p->dy * W + p->dx + p->dw - 1];
	CHECK(!back && first == 16 && last == 235, "%dx%d ramps: %d steps backwards, ends %u..%u", w, h, back, first, last);
	free(f); free(slot);
}

static void passthrough(void)
{
	static struct geometry g;
	geometry_setup(&g, W, H, 4.0 / 3.0);
	uint8_t *f = frame_new(W, H), *slot = malloc(SLOT_BYTES);
	fill_random(f, W, H, 7);
	scale_frame(&g, f, slot);
	CHECK(!memcmp(f, slot, SLOT_BYTES), "720x480 4:3 frame is not passed through unchanged");
	free(f); free(slot);
}

static void neon_matches_c(int w, int h, double aspect)
{
	static struct geometry g;
	geometry_setup(&g, w, h, aspect);
	uint8_t *f = frame_new(w, h), *a = malloc(SLOT_BYTES), *b = malloc(SLOT_BYTES);
	fill_random(f, w, h, (unsigned)(w * 31 + h));
	g_scale_c = 0; scale_frame(&g, f, a);
	g_scale_c = 1; scale_frame(&g, f, b);
	g_scale_c = 0;
	size_t diff = 0;
	for (size_t i = 0; i < SLOT_BYTES; i++) diff += a[i] != b[i];
	CHECK(!diff, "%dx%d: NEON and C differ in %zu bytes", w, h, diff);
	free(f); free(a); free(b);
}

static void timing(int w, int h, double aspect)
{
	static struct geometry g;
	geometry_setup(&g, w, h, aspect);
	uint8_t *f = frame_new(w, h), *slot = malloc(SLOT_BYTES);
	fill_random(f, w, h, 3);
	for (int c = 0; c < 2; c++) {
		g_scale_c = c;
		double t = now();
		for (int i = 0; i < 50; i++) scale_frame(&g, f, slot);
		printf("  %dx%d -> %dx%d: %s %.2f ms/frame\n", w, h, g.p[0].dw, g.p[0].dh, c ? "C   " : "fast", (now() - t) * 1e3 / 50);
	}
	g_scale_c = 0;
	free(f); free(slot);
}

int main(int argc, char **argv)
{
	/* frame shapes Plex sends, and a few it could */
	fits(640, 480, 640.0 / 480, 0, 0, 720, 480);        /* 4:3 */
	fits(720, 480, 4.0 / 3.0, 0, 0, 720, 480);          /* 4:3 DVD, aspect from the header */
	fits(644, 480, 644.0 / 480, 0, 0, 720, 480);        /* a hair wider than 4:3: lines kept 1:1 */
	fits(636, 480, 636.0 / 480, 0, 0, 720, 480);        /* a hair narrower: no side slivers */
	fits(720, 404, 720.0 / 404, 0, 60, 720, 360);       /* 16:9 */
	fits(720, 480, 16.0 / 9.0, 0, 60, 720, 360);        /* anamorphic 16:9 DVD */
	fits(720, 306, 720.0 / 306, 0, 104, 720, 272);      /* 2.35:1 */
	fits(480, 360, 480.0 / 360, 0, 0, 720, 480);        /* small 4:3 */
	fits(576, 480, 1.2, 36, 0, 648, 480);               /* narrower than 4:3: pillarbox */
	fits(270, 480, 270.0 / 480, 208, 0, 304, 480);      /* portrait */
	fits(640, 480, 0, 0, 0, 720, 480);                  /* no aspect in the header: square pixels */

	passthrough();
	int shapes[][2] = { {640, 480}, {644, 480}, {720, 404}, {720, 306}, {480, 360}, {576, 480}, {270, 480},
	                    {642, 482}, {1280, 720}, {352, 240} };
	for (unsigned i = 0; i < sizeof shapes / sizeof *shapes; i++) {
		int w = shapes[i][0], h = shapes[i][1];
		flat_and_borders(w, h, (double)w / h);
		ramps(w, h, (double)w / h);
		neon_matches_c(w, h, (double)w / h);
	}
	neon_matches_c(720, 480, 16.0 / 9.0);
#ifdef HAVE_NEON
	printf("NEON build\n");
#endif
	if (argc > 1 && !strcmp(argv[1], "time")) {
		timing(640, 480, 4.0 / 3.0);
		timing(644, 480, 644.0 / 480);
		timing(720, 404, 720.0 / 404);
		timing(640, 360, 16.0 / 9.0);
	}
	printf("%d checks, %d failed\n", checks, fails);
	return fails != 0;
}
