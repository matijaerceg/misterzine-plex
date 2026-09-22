/*
 * plexui - drawing and input helper for the PlexCRT menu.
 *
 * The menu logic lives in Python (plexmenu.py); this process does the two
 * things Python is bad at on a 800 MHz ARM: blitting a 720x480 frame with
 * antialiased text quickly, and polling the core's input words at 60 Hz.
 *
 * Frame format: xRGB8888 (core fmt 0), 720x480, ring slots as in plexfb.
 * Everything is drawn into a malloc'd back buffer and copied to a free ring
 * slot on `show`, so text blending never reads write-combined memory.
 *
 * stdin, one command per line:
 *   fill RRGGBB
 *   rect X Y W H RRGGBB
 *   text X Y RRGGBB big|small TEXT...        (Y = top of the glyph cell)
 *   img X Y PATH                             binary PPM (P6) at its own size
 *   width big|small TEXT...                  -> "width N" on stdout
 *   show                                     publish the back buffer
 *   blank                                    publish a black frame
 *   quit
 * stdout, unsolicited:
 *   joy HHHH                                 joystick_1<<16 | joystick_0, on change
 *   key CODE P                               ps2 scan code (0xE0.. for extended), P=1 press
 * Both need core build 15 or later (input snapshot at BASE+0x48).
 *
 * Build (WSL): arm-linux-gnueabihf-gcc -O2 -static -march=armv7-a -mfpu=neon
 *              -mfloat-abi=hard -o plexui plexui.c -lpthread
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <pthread.h>
#include <signal.h>
#include "font.h"

#define NSLOT       5
#define BUF_OFF(i)  (0x100000u + (uint32_t)(i) * 0x160000u)
#define STAT_OFF    0x40u
#define MAGIC       0x504C4558u
#define W 720
#define H 480

struct hdr { volatile uint32_t magic, seq, buf, width, height, stride, fmt, u_off, v_off, flags; };
struct stat_w { volatile uint32_t field_cnt, seq_shown, joy, key; };

static uint8_t *map;
static struct hdr *hdr;
static struct stat_w *stat;
static uint32_t *bb;                 /* back buffer */
static volatile sig_atomic_t g_quit = 0;
static void on_term(int s) { (void)s; g_quit = 1; }

static void map_mem(void)
{
	int fd = open("/dev/fb0", O_RDWR | O_CLOEXEC);
	if (fd < 0) { perror("/dev/fb0"); exit(1); }
	map = mmap(0, 0x7E0000u, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (map == MAP_FAILED) { perror("mmap"); exit(1); }
	hdr  = (struct hdr *)map;
	stat = (struct stat_w *)(map + STAT_OFF);
}

static void publish(int slot, uint32_t seq)
{
	hdr->magic = MAGIC; hdr->buf = slot; hdr->width = W; hdr->height = H;
	hdr->stride = W * 4; hdr->fmt = 0; hdr->u_off = 0; hdr->v_off = 0; hdr->flags = 0;
	__sync_synchronize();
	hdr->seq = seq;
}

static void show(void)
{
	/* the slot after the one currently published is never on screen, whoever
	   published it (plexfb leaves its black frame behind when playback ends) */
	int slot = (hdr->magic == MAGIC) ? (int)((hdr->buf + 1) % NSLOT) : 0;
	memcpy(map + BUF_OFF(slot), bb, W * H * 4);
	__sync_synchronize();
	publish(slot, hdr->seq + 1);
}

/* ---------- drawing ---------- */
static void fill(uint32_t c) { for (int i = 0; i < W * H; i++) bb[i] = c; }

static void rect(int x, int y, int w, int h, uint32_t c)
{
	int x0 = x < 0 ? 0 : x, y0 = y < 0 ? 0 : y;
	int x1 = x + w > W ? W : x + w, y1 = y + h > H ? H : y + h;
	for (int yy = y0; yy < y1; yy++)
		for (int xx = x0; xx < x1; xx++) bb[yy * W + xx] = c;
}

static inline void blend(uint32_t *d, uint32_t c, unsigned a)
{
	if (!a) return;
	if (a >= 255) { *d = c; return; }
	uint32_t s = *d;
	unsigned r = ((s >> 16) & 255), g = ((s >> 8) & 255), b = (s & 255);
	unsigned cr = ((c >> 16) & 255), cg = ((c >> 8) & 255), cb = (c & 255);
	r += ((int)(cr - r) * (int)a) / 255;
	g += ((int)(cg - g) * (int)a) / 255;
	b += ((int)(cb - b) * (int)a) / 255;
	*d = (r << 16) | (g << 8) | b;
}

struct font { const unsigned char *px; const unsigned char *adv; int cw, ch; };
static const struct font FBIG   = { &font_big[0][0],   font_big_adv,   font_big_W,   font_big_H };
static const struct font FSMALL = { &font_small[0][0], font_small_adv, font_small_W, font_small_H };

static int text_width(const struct font *f, const char *s)
{
	int w = 0;
	for (; *s; s++) { int c = (unsigned char)*s; if (c < 32 || c > 126) c = '?'; w += f->adv[c - 32]; }
	return w;
}

static void text(const struct font *f, int x, int y, uint32_t col, const char *s)
{
	for (; *s; s++) {
		int c = (unsigned char)*s;
		if (c < 32 || c > 126) c = '?';
		const unsigned char *g = f->px + (c - 32) * f->cw * f->ch;
		for (int yy = 0; yy < f->ch; yy++) {
			int py = y + yy;
			if (py < 0 || py >= H) continue;
			for (int xx = 0; xx < f->cw; xx++) {
				int px = x + xx - 1;                 /* glyphs were drawn at cell x=1 */
				if (px < 0 || px >= W) continue;
				blend(&bb[py * W + px], col, g[yy * f->cw + xx]);
			}
		}
		x += f->adv[c - 32];
	}
}

static void img(int x, int y, const char *path)
{
	FILE *fp = fopen(path, "rb");
	if (!fp) { fprintf(stderr, "plexui: %s: cannot open\n", path); return; }
	char m[3] = {0}; int w, h, mx;
	if (fscanf(fp, "%2s %d %d %d", m, &w, &h, &mx) != 4 || strcmp(m, "P6") || mx != 255) {
		fprintf(stderr, "plexui: %s: not a P6 ppm\n", path); fclose(fp); return;
	}
	fgetc(fp);                                        /* single whitespace after maxval */
	uint8_t *row = malloc(w * 3);
	for (int yy = 0; yy < h; yy++) {
		if (fread(row, 1, w * 3, fp) != (size_t)w * 3) break;
		int py = y + yy;
		if (py < 0 || py >= H) continue;
		for (int xx = 0; xx < w; xx++) {
			int px = x + xx;
			if (px < 0 || px >= W) continue;
			bb[py * W + px] = (row[xx * 3] << 16) | (row[xx * 3 + 1] << 8) | row[xx * 3 + 2];
		}
	}
	free(row); fclose(fp);
}

/* ---------- input ---------- */
static void *input_thread(void *arg)
{
	(void)arg;
	uint32_t joy_q = stat->joy, key_q = stat->key;
	while (!g_quit) {
		uint32_t j = stat->joy, k = stat->key;
		if (j != joy_q) { joy_q = j; printf("joy %08x\n", j); fflush(stdout); }
		if ((k ^ key_q) >> 16) {                      /* key_cnt moved: a new event */
			key_q = k;
			printf("key %02x%02x %d\n", (k >> 8) & 1 ? 0xE0 : 0, k & 0xFF, (k >> 9) & 1);
			fflush(stdout);
		}
		usleep(4000);
	}
	return NULL;
}

int main(void)
{
	map_mem();
	bb = calloc(W * H, 4);
	signal(SIGTERM, on_term);
	signal(SIGINT, on_term);
	signal(SIGPIPE, SIG_IGN);
	pthread_t ti;
	pthread_create(&ti, NULL, input_thread, NULL);

	char line[4096];
	while (!g_quit && fgets(line, sizeof line, stdin)) {
		line[strcspn(line, "\r\n")] = 0;
		char cmd[16]; int n = 0;
		if (sscanf(line, "%15s%n", cmd, &n) < 1) continue;
		const char *rest = line + n;
		if (!strcmp(cmd, "fill")) { unsigned c; if (sscanf(rest, "%x", &c) == 1) fill(c); }
		else if (!strcmp(cmd, "rect")) {
			int x, y, w, h; unsigned c;
			if (sscanf(rest, "%d %d %d %d %x", &x, &y, &w, &h, &c) == 5) rect(x, y, w, h, c);
		}
		else if (!strcmp(cmd, "text")) {
			int x, y, m; unsigned c; char f[8];
			if (sscanf(rest, "%d %d %x %7s%n", &x, &y, &c, f, &m) == 4)
				text(f[0] == 'b' ? &FBIG : &FSMALL, x, y, c, rest + m + (rest[m] == ' '));
		}
		else if (!strcmp(cmd, "width")) {
			int m; char f[8];
			if (sscanf(rest, "%7s%n", f, &m) == 1) {
				printf("width %d\n", text_width(f[0] == 'b' ? &FBIG : &FSMALL, rest + m + (rest[m] == ' ')));
				fflush(stdout);
			}
		}
		else if (!strcmp(cmd, "img")) {
			int x, y, m;
			if (sscanf(rest, "%d %d%n", &x, &y, &m) == 2) img(x, y, rest + m + (rest[m] == ' '));
		}
		else if (!strcmp(cmd, "show")) show();
		else if (!strcmp(cmd, "blank")) { fill(0); show(); }
		else if (!strcmp(cmd, "quit")) break;
		else fprintf(stderr, "plexui: unknown command %s\n", cmd);
	}
	g_quit = 1;
	pthread_join(ti, NULL);
	return 0;
}
