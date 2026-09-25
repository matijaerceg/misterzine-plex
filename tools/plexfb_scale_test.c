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
	      "%dx%d aspect %.4f in %d,%d,%d,%d width %d -> %dx%d at %d,%d, expected %dx%d at %d,%d",
	      w, h, aspect, g_screen.l, g_screen.t, g_screen.r, g_screen.b, g_screen.width,
	      p->dw, p->dh, p->dx, p->dy, ew, eh, ex, ey);
	const struct plane_map *u = &g.p[1];
	CHECK(u->dx == ex / 2 && u->dy == ey / 2 && u->dw == ew / 2 && u->dh == eh / 2,
	      "%dx%d chroma rectangle %dx%d at %d,%d", w, h, u->dw, u->dh, u->dx, u->dy);
}

/* tools/testdata/fit_cases.txt, which the app's Go tests read too (found
   beside this source, or at PLEXFB_FIT_CASES when the test runs elsewhere) */
static void fit_cases(void)
{
	char path[1024];
	const char *slash = strrchr(__FILE__, '/');
	snprintf(path, sizeof path, "%.*stestdata/fit_cases.txt", slash ? (int)(slash - __FILE__ + 1) : 0, __FILE__);
	if (getenv("PLEXFB_FIT_CASES")) snprintf(path, sizeof path, "%s", getenv("PLEXFB_FIT_CASES"));
	FILE *f = fopen(path, "r");
	CHECK(f != NULL, "cannot open %s", path);
	if (!f) return;
	char line[256];
	int n = 0;
	while (fgets(line, sizeof line, f)) {
		struct screen s;
		double num, den;
		int lines, x, y, w, h;
		if (sscanf(line, "%d %d %d %d %d %lf/%lf %d %d %d %d %d", &s.l, &s.t, &s.r, &s.b, &s.width,
		           &num, &den, &lines, &x, &y, &w, &h) != 12) continue;
		char text[64];
		snprintf(text, sizeof text, "%d,%d,%d,%d,%d", s.l, s.t, s.r, s.b, s.width);
		CHECK(parse_screen(text, &g_screen), "%s: not accepted", text);
		/* a frame of that many lines; its width does not move it */
		int fw = 2 * (int)lround(lines * num / den / 2);
		fits(fw < 16 ? 16 : fw > MAX_SRC_W ? MAX_SRC_W : fw, lines, num / den, x, y, w, h);
		n++;
	}
	fclose(f);
	CHECK(n >= 30, "only %d fit cases read from %s", n, path);
	g_screen = (struct screen){ 0, 0, 0, 0, 1000 };
}

/* fit() as it was before calibration (commit 71cfd60^) */
static void old_fit(double aspect, int sh, int *dx, int *dy, int *dw, int *dh)
{
	int w = W, h = H;
	if (aspect > 4.0 / 3.0) h = 2 * (int)lround(H * (4.0 / 3.0) / aspect / 2);
	else                    w = 2 * (int)lround(W * aspect / (4.0 / 3.0) / 2);
	if (w < 16) w = 16;
	if (h < 16) h = 16;
	if (w >= W - W / 60) w = W;
	if (!(sh & 1) && sh <= H && abs(h - sh) <= H / 50) h = sh;
	*dw = w; *dh = h; *dx = (W - w) / 2 & ~1; *dy = (H - h) / 2 & ~1;
}

/* Without calibration every frame Plex can send (even sizes up to 720x480,
   square pixels) lands where it did before. The one known difference: a
   frame a hair wider than 4:3 with far fewer than 480 lines (484x360) now
   fills the screen like 4:3 instead of leaving 2-4 lines of border. */
static void default_unchanged(void)
{
	g_screen = (struct screen){ 0, 0, 0, 0, 1000 };
	int same = 0, known = 0, other = 0;
	for (int sh = 16; sh <= H; sh += 2)
		for (int sw = 16; sw <= W; sw += 2) {
			double a = (double)sw / sh;
			int ox, oy, ow, oh, x, y, w, h;
			old_fit(a, sh, &ox, &oy, &ow, &oh);
			fit(&g_screen, a, sh, &x, &y, &w, &h);
			if (ox == x && oy == y && ow == w && oh == h) { same++; continue; }
			if (a > 4.0 / 3.0 && a * 3 / 4 <= 61.0 / 60 && w == W && h == H) { known++; continue; }
			if (other++ < 5) printf("  %dx%d: was %dx%d at %d,%d, now %dx%d at %d,%d\n", sw, sh, ow, oh, ox, oy, w, h, x, y);
		}
	CHECK(!other, "default placement changed for %d frame sizes (%d unchanged, %d known)", other, same, known);
}

static void screen_settings(void)
{
	struct screen s = { 0, 0, 0, 0, 1000 };
	const char *bad[] = { "", "1,0,0,0,1000", "0,0,0,0,849", "0,0,0,0,1151", "122,0,0,0,1000",
	                      "0,82,0,0,1000", "-2,0,0,0,1000", "0,0,0,0", "0,0,0,0,1000x", "0,0,0,0,1000,4" };
	for (unsigned i = 0; i < sizeof bad / sizeof *bad; i++)
		CHECK(!parse_screen(bad[i], &s) && s.width == 1000, "PLEXFB_GEOMETRY \"%s\" accepted", bad[i]);
	CHECK(parse_screen("120,80,120,80,1150", &s) && s.l == 120 && s.b == 80 && s.width == 1150, "the limits refused");
	CHECK(parse_screen("0,0,0,0,850", &s) && s.width == 850, "the narrowest width refused");
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

static void wr32(uint8_t *p, uint32_t v) { p[0] = v; p[1] = v >> 8; p[2] = v >> 16; p[3] = v >> 24; }

static int strf(int32_t w, int32_t h, int bits, const char *fourcc, uint32_t sz, int *ow, int *oh)
{
	uint8_t b[40] = { 0 };
	wr32(b, 40); wr32(b + 4, (uint32_t)w); wr32(b + 8, (uint32_t)h);
	b[12] = 1; b[14] = (uint8_t)bits; memcpy(b + 16, fourcc, 4);
	*ow = *oh = -1;
	return parse_strf(b, sz, ow, oh);
}

static void headers(void)
{
	int w, h;
	CHECK(strf(644, 480, 12, "I420", 40, &w, &h) && w == 644 && h == 480, "I420 644x480 not taken: %dx%d", w, h);
	CHECK(strf(640, -480, 12, "IYUV", 40, &w, &h) && w == 640 && h == 480, "top-down IYUV not taken: %dx%d", w, h);
	CHECK(!strf(640, INT32_MIN, 12, "I420", 40, &w, &h), "height -2^31 taken");
	CHECK(!strf(640, 480, 12, "YV12", 40, &w, &h), "YV12 (swapped planes) taken");
	CHECK(!strf(640, 480, 24, "I420", 40, &w, &h), "24 bits taken");
	CHECK(!strf(4096, 2160, 12, "I420", 40, &w, &h), "4096x2160 taken");
	CHECK(!strf(8, 8, 12, "I420", 40, &w, &h), "8x8 taken");
	CHECK(!strf(640, 480, 12, "I420", 16, &w, &h), "short strf taken");
	uint8_t v[36] = { 0 };
	wr32(v + 20, 16u << 16 | 9);
	CHECK(fabs(parse_vprp(v, 36) - 16.0 / 9.0) < 1e-9, "vprp 16:9 read as %f", parse_vprp(v, 36));
	CHECK(parse_vprp(v, 20) == 0, "short vprp read");
	wr32(v + 20, 0);
	CHECK(parse_vprp(v, 36) == 0, "vprp without an aspect read");
}

/* a crop rectangle (a zoom, later): only the rectangle's pixels reach the screen */
static void crop(void)
{
	static struct plane_map m;
	int w = 640, h = 360;
	uint8_t *f = frame_new(w, h), *out = malloc(W * H);
	for (int r = 0; r < h; r++)
		for (int x = 0; x < w; x++) f[(size_t)r * w + x] = (uint8_t)(x * 255 / (w - 1));
	/* the middle 480x360 of a 16:9 frame, filling the 4:3 screen */
	plane_setup(&m, w, h, 80, 0, 480, 360, 0, 0, W, H, W, H, 16);
	scale_plane(&m, f, out);
	uint8_t lo = f[80], hi = f[80 + 479];
	int outside = 0, back = 0;
	for (int r = 0; r < H; r++)
		for (int x = 0; x < W; x++) {
			uint8_t v = out[r * W + x];
			if (v < lo || v > hi) outside++;
			if (x && v < out[r * W + x - 1]) back++;
		}
	CHECK(!outside && !back && out[0] == lo && out[W - 1] == hi, "crop: %d samples outside %u..%u, %d backwards, ends %u..%u",
	      outside, lo, hi, back, out[0], out[W - 1]);
	free(f); free(out);
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
	/* frame shapes Plex sends and a few it could, on the whole raster and in
	   calibrated picture areas */
	fit_cases();
	default_unchanged();
	fits(640, 480, 0, 0, 0, 720, 480);                  /* no aspect in the header: square pixels */
	fits(720, 480, 4.0 / 3.0, 0, 0, 720, 480);          /* 4:3 DVD, aspect from the header */
	fits(720, 480, 16.0 / 9.0, 0, 60, 720, 360);        /* anamorphic 16:9 DVD */
	screen_settings();

	headers();
	crop();
	passthrough();
	int shapes[][2] = { {640, 480}, {644, 480}, {720, 404}, {720, 306}, {480, 360}, {576, 480}, {270, 480},
	                    {642, 482}, {641, 481}, {1280, 720}, {352, 240} };
	/* the whole raster, then a calibrated area: edges in, both axes resampled */
	struct screen areas[] = { { 0, 0, 0, 0, 1000 }, { 16, 12, 18, 10, 985 } };
	for (unsigned a = 0; a < sizeof areas / sizeof *areas; a++) {
		g_screen = areas[a];
		for (unsigned i = 0; i < sizeof shapes / sizeof *shapes; i++) {
			int w = shapes[i][0], h = shapes[i][1];
			flat_and_borders(w, h, (double)w / h);
			ramps(w, h, (double)w / h);
			neon_matches_c(w, h, (double)w / h);
		}
		neon_matches_c(720, 480, 16.0 / 9.0);
	}
	g_screen = areas[0];
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
