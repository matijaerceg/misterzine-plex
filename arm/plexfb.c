/*
 * SPDX-License-Identifier: GPL-3.0-or-later
 * Copyright 2026 Matija Erceg
 *
 * plexfb - ARM-side writer for the PlexCRT core's DDR3 scan-out.
 *
 * Memory layout: see rtl/ddr_scanout.v. Everything lives at physical
 * 0x30000000, in the region the kernel leaves alone (mem=511M on MiSTer).
 *
 *   plexfb card             draw a static test card into buffer 0 and exit
 *   plexfb anim [seconds]   moving bars, alternating buffers, paced on the
 *                           core's field counter (double-buffer / tear test)
 *   plexfb raw [fps] [yuv]  read 720x480 frames from stdin and show them:
 *                           BGRA (ffmpeg ... -f rawvideo -pix_fmt bgra -) or,
 *                           with "yuv", planar yuv420p converted in the core
 *   plexfb status           print the header and status words
 *
 * Build (WSL): arm-linux-gnueabihf-gcc -O2 -static -march=armv7-a -mfpu=neon
 *              -mfloat-abi=hard -o plexfb plexfb.c
 */
#define _GNU_SOURCE          /* F_SETPIPE_SZ */
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <math.h>
#include <sys/mman.h>
#include <time.h>
#include <pthread.h>
#include <signal.h>
#include <sys/types.h>

#define PHYS_BASE   0x30000000u
#define MAP_SIZE    (8u << 20)
#define NSLOT       4                                   /* frame ring, see ddr_scanout.v */
#define RING        4                                   /* slots the video cycles through */
#define BUF_OFF(i)  (0x100000u + (uint32_t)(i) * 0x160000u)
#define STAT_OFF    0x40u
#define MAGIC       0x504C4558u

#define W       720
#define H       480
#define STRIDE  (W * 4)

struct hdr {
	volatile uint32_t magic, seq, buf, width, height, stride, fmt, u_off, v_off, flags;
};

/* frame format: 0 = xRGB8888, 1 = planar yuv420p (ffmpeg -pix_fmt yuv420p) */
static int g_yuv = 0;
#define FRAME_BYTES   (g_yuv ? W * H * 3 / 2 : W * H * 4)
#define Y_STRIDE      W
#define U_OFF_WORDS   (W * H / 8)                 /* 43200 */
#define V_OFF_WORDS   (W * H / 8 + (W / 2) * (H / 2) / 8)   /* 54000 */
struct stat_w {
	volatile uint32_t field_cnt, seq_shown;
	volatile uint32_t joy;      /* build 15+: joystick_1[15:0] << 16 | joystick_0[15:0] */
	volatile uint32_t key;      /* build 15+: key_cnt << 16 | ps2_key[10:0] */
};

/* runtime control (AVI / raw modes):
 *   SIGUSR1  toggle pause (picture holds, audio stops with it)
 *   SIGTERM  leave cleanly: blank the screen, exit 0
 *   SIGUSR2  leave quietly: keep the last frame on screen (a seek: the next
 *            presenter takes over from it and starts its ring past that slot)
 * Progress goes to PLEXFB_STATUS (default /tmp/plexfb.stat) twice a second:
 *   pos=<s> shown=<n> filled=<n> eof=<0|1> paused=<0|1> starved=<n>
 */
static volatile sig_atomic_t g_pause = 0, g_quit = 0, g_hold = 0;
static void on_usr1(int s) { (void)s; g_pause = !g_pause; }
static void on_term(int s) { (void)s; g_quit = 1; }
static void on_usr2(int s) { (void)s; g_hold = 1; g_quit = 1; }

static uint8_t *map;
static struct hdr *hdr;
static struct stat_w *stat;

/*
 * Two ways to reach the buffer:
 *   /dev/mem at PHYS_BASE  - strongly ordered mapping, every store waits
 *                            (measured 87 MB/s idle, 44 MB/s next to ffmpeg)
 *   /dev/fb0 at offset 0   - the fbdev driver maps its memory write-combined,
 *                            which is what we want for streaming frames.
 * PLEXFB_DEV=/dev/fb0 selects the second; the core must then read from the
 * framebuffer's physical address instead of PHYS_BASE.
 */
static void map_mem(void)
{
	const char *dev = getenv("PLEXFB_DEV");
	int fd;
	off_t off;
	if (dev && !strcmp(dev, "/dev/mem")) {          /* legacy, build 7 layout */
		fd = open(dev, O_RDWR | O_SYNC | O_CLOEXEC);
		off = PHYS_BASE;
	} else {
		if (!dev) dev = "/dev/fb0";                 /* default since build 8 */
		fd = open(dev, O_RDWR | O_CLOEXEC);
		off = 0;
	}
	if (fd < 0) { perror(dev); exit(1); }
	/* fb0 is 1920*1080*4 = 8,294,400 bytes; mmap refuses anything longer */
	size_t len = off ? MAP_SIZE : 0x7E0000u;   /* slot 4 ends at 0x7D1800 */
	map = mmap(0, len, PROT_READ | PROT_WRITE, MAP_SHARED, fd, off);
	if (map == MAP_FAILED) { perror("mmap"); exit(1); }
	hdr  = (struct hdr *)(map);
	stat = (struct stat_w *)(map + STAT_OFF);
}

/* publish buffer `buf` as frame `seq`: header first, seq last */
static void publish(int buf, uint32_t seq)
{
	hdr->magic  = MAGIC;
	hdr->buf    = buf;
	hdr->width  = W;
	hdr->height = H;
	hdr->stride = g_yuv ? Y_STRIDE : STRIDE;
	hdr->fmt    = g_yuv;
	hdr->u_off  = U_OFF_WORDS;
	hdr->v_off  = V_OFF_WORDS;
	hdr->flags  = 0;
	__sync_synchronize();
	hdr->seq    = seq;
}

static void wait_field(uint32_t *last)
{
	for (;;) {
		uint32_t f = stat->field_cnt;
		if (f != *last) { *last = f; return; }
		usleep(300);
	}
}

static double now(void)
{
	struct timespec ts; clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec + ts.tv_nsec * 1e-9;
}

/* Block until field_cnt reaches target. The kernel's sleep granularity is
   coarse enough to overshoot a whole field, which turns a 3:2 cadence into
   4:1, so this sleeps only to a few ms before the field is due and spins the
   rest, watching the counter the core writes every vsync. */
static double g_fps, g_avi_fps;      /* set in avi mode; defined below */
#define FIELD_S (1.0 / 59.94)
static void wait_for_field(double target)
{
	static uint32_t last; static double last_t;
	for (;;) {
		uint32_t f = stat->field_cnt;
		double t = now();
		if (f != last) { last = f; last_t = t; }
		if ((double)f >= target || g_quit) return;
		double eta = last_t + (target - f) * FIELD_S - t;   /* seconds to go */
		if (eta > 0.0035) usleep((useconds_t)((eta - 0.0035) * 1e6));
		else if (eta > 0.0003) usleep(100);
		/* else spin */
	}
}

/* ---------- test card ---------- */
static const uint32_t bars[8] = {
	0xEBEBEB, 0xEBEB10, 0x10EBEB, 0x10EB10, 0xEB10EB, 0xEB1010, 0x1010EB, 0x101010
};

static void draw_card(uint32_t *f)
{
	for (int y = 0; y < H; y++) {
		for (int x = 0; x < W; x++) {
			uint32_t c;
			if (y < 120)       c = bars[x * 8 / W];
			else if (y < 240) {                     /* grid 16 px, one-pixel lines */
				c = ((x & 15) == 0 || (y & 15) == 0) ? 0xEBEBEB : 0x101010;
			}
			else if (y < 360) {                     /* ring + X, same as the FPGA card */
				int dx = x - 360, dy = y - 300;
				int d2 = dx * dx + dy * dy;
				c = 0x101010;
				if (d2 > 2966 && d2 < 3306) c = 0x10EBEB;
				if (abs(x - (y + 60)) <= 1 || abs(x - (660 - y)) <= 1) c = 0xEBEB10;
			}
			else {                                  /* grey ramp */
				uint32_t v = x * 255 / (W - 1);
				c = (v << 16) | (v << 8) | v;
			}
			/* 8-px border, blue, so the ARM frame is told apart from the FPGA card */
			if (x < 8 || x >= W - 8 || y < 4 || y >= H - 4) c = 0x2020FF;
			f[y * W + x] = c;
		}
	}
}

static void draw_anim(uint32_t *f, int t)
{
	int bx = (t * 6) % (W - 40);           /* a bar sweeping right, 6 px per field */
	int by = (t * 3) % (H - 40);           /* a bar sweeping down, 3 rows per field */
	for (int y = 0; y < H; y++) {
		for (int x = 0; x < W; x++) {
			uint32_t c = ((x >> 5) + (y >> 5)) & 1 ? 0x303030 : 0x181818;
			if (x >= bx && x < bx + 40) c = 0xEB4040;
			if (y >= by && y < by + 40) c = 0x40EB40;
			if (x < 8 || x >= W - 8 || y < 4 || y >= H - 4) c = 0x2020FF;
			f[y * W + x] = c;
		}
	}
}

static void copy_frame(int buf, const uint32_t *src)
{
	memcpy(map + BUF_OFF(buf), src, W * H * 4);
}

/* ---------- ring-buffered presenter ----------
 * The reader thread pulls frames from stdin straight into free ring slots and
 * can run up to NSLOT-2 frames ahead of the screen. The presenter publishes
 * one frame per `fields_per_frame` fields, never blocking on the decoder, so
 * decoder jitter is absorbed instead of turning into repeated frames. */
static volatile unsigned ring_filled = 0, ring_shown = 0;   /* frame counters */
/* the counters start at ring_base so the first slots written are not the
   one a previous presenter left on screen (its frame holds until ours) */
static unsigned ring_base = 0;
static volatile int ring_eof = 0;
static double stat_read_ms = 0;
/* every decoded frame is kept in RAM too (the ring is write-combined and
   must never be read): the overlay blends there, and a pause holds it */
static uint8_t *ram[RING];
static uint8_t *ram_slot(int i) { if (!ram[i]) ram[i] = malloc(W * H * 4); return ram[i]; }

static void *reader_thread(void *arg)
{
	(void)arg;
	size_t need = FRAME_BYTES;
	for (;;) {
		/* slot (filled % NSLOT) is free once the frame that used it is no
		   longer on screen. After publish(shown) the core still shows frame
		   shown-1 until the next vsync, so frames shown-1 AND shown-2 must
		   stay untouched: never run more than NSLOT-2 frames ahead. */
		while (ring_filled - ring_shown >= RING - 2) usleep(500);
		uint8_t *dst = ram_slot(ring_filled % RING);
		size_t got = 0;
		double a = now();
		while (got < need) {
			ssize_t r = read(0, dst + got, need - got);
			if (r <= 0) { ring_eof = 1; return NULL; }
			got += r;
		}
		stat_read_ms += (now() - a) * 1e3;
		memcpy(map + BUF_OFF(ring_filled % RING), dst, need);
		__sync_synchronize();
		ring_filled++;
	}
}

/* progress for the launcher (timeline pings, resume point), written atomically */
static void write_status(double pos, int starved)
{
	static const char *path;
	static char tmp[256];
	if (!path) {
		path = getenv("PLEXFB_STATUS") ? getenv("PLEXFB_STATUS") : "/tmp/plexfb.stat";
		snprintf(tmp, sizeof tmp, "%s.tmp", path);
	}
	FILE *f = fopen(tmp, "w");
	if (!f) return;
	fprintf(f, "pos=%.2f shown=%u filled=%u eof=%d paused=%d starved=%d\n",
	        pos, ring_shown - ring_base, ring_filled - ring_base, ring_eof, (int)g_pause, starved);
	fclose(f);
	rename(tmp, path);
}

/* black frame into a slot the core is not showing, then publish it: leaves
   the CRT dark instead of holding the last picture after the stream ends */
static void blank_screen(uint32_t seq)
{
	int slot = (ring_shown + 1) % RING;
	uint8_t *dst = map + BUF_OFF(slot);
	if (g_yuv) {
		memset(dst, 16, W * H);
		memset(dst + W * H, 128, W * H / 2);
	} else
		memset(dst, 0, W * H * 4);
	__sync_synchronize();
	publish(slot, seq);
}

static void present_loop(double fps, uint32_t seq)
{
	/* let the reader get a head start so the first frames are not starved */
	while (ring_filled - ring_base < RING - 2 && !ring_eof && !g_quit) usleep(1000);
	if (g_avi_fps > 10 && g_avi_fps < 70 && fabs(g_avi_fps - fps) > 0.01) {
		fprintf(stderr, "plexfb: stream says %.3f fps, launcher said %.3f: following the stream\n", g_avi_fps, fps);
		fps = g_fps = g_avi_fps;
	}
	double fields_per_frame = 59.94 / fps;
	double target = stat->field_cnt + 1;
	int n = 0, starved = 0, max_ahead = 0;
	double t0 = now(), tstat = 0;
	/* the playback overlay is the core's own plane now (see ddr_scanout.v):
	   the presenter only publishes picture frames */
	int last_slot = -1;
	for (;;) {
		wait_for_field(target);
		if (g_quit) break;
		if (now() - tstat > 0.5) { tstat = now(); write_status((ring_shown - ring_base) / fps, starved); }
		if (g_pause) {                 /* hold: keep the clock pinned to now */
			target = stat->field_cnt + 1;
			continue;
		}
		if (ring_filled == ring_shown) {
			if (ring_eof) break;
			starved++;                 /* decoder behind: frame repeats */
			target += fields_per_frame;
			continue;
		}
		int ahead = ring_filled - ring_shown;
		if (ahead > max_ahead) max_ahead = ahead;
		last_slot = ring_shown % RING;
		publish(last_slot, ++seq);
		ring_shown++;
		n++;
		target += fields_per_frame;
		if ((double)stat->field_cnt > target + 1.0)   /* fell more than a frame behind */
			target = stat->field_cnt + 1;
	}
	double dt = now() - t0;
	ring_eof = 1;                      /* tells the audio thread to wind down */
	write_status((ring_shown - ring_base) / fps, starved);
	if (!g_hold) blank_screen(++seq);
	printf("%d frames in %.1fs (%.2f fps), avg read %.1f ms, %d starved slots, max %d frames ahead%s\n",
	       n, dt, n / dt, n ? stat_read_ms / n : 0, starved, max_ahead, g_quit ? ", stopped" : "");
}

/* ---------- AVI mode: one interleaved stream, the presenter is the clock ----------
 * ffmpeg writes raw yuv420p video ('00dc' chunks) and S16LE stereo 48 kHz audio
 * ('01wb' chunks) into a single AVI on stdin. Video goes into the frame ring as
 * before. Audio goes into a FIFO and is released to aplay only when the picture
 * has caught up to it, so the audio position follows the field counter and does
 * not depend on how far ahead the source interleaved its audio (Plex sends it
 * about a second early, which is what put the sound ahead of the picture).
 *
 *   PLEXFB_ALEAD  seconds of audio kept queued ahead of the picture (default 0.10,
 *                 roughly aplay's buffer so it never runs dry)
 *   PLEXFB_AOFF   extra audio delay in seconds, positive = later (default 0.08)
 */
#define AFIFO_BYTES (48000 * 4 * 8)          /* 8 s of S16 stereo */
#define ABPS        (48000.0 * 4)
static uint8_t *afifo;
static volatile size_t a_w = 0, a_r = 0;     /* monotonic byte counters */
static double g_alead = 0.10, g_aoff = 0.08;   /* 80 ms: measured on the CRT with the sweep clip */
static FILE *aplay;

static uint32_t rd32(const uint8_t *p) { return p[0] | p[1] << 8 | p[2] << 16 | (uint32_t)p[3] << 24; }

static int read_full(void *dst, size_t n)
{
	size_t got = 0;
	while (got < n) {
		ssize_t r = read(0, (uint8_t *)dst + got, n - got);
		if (r <= 0) return 0;
		got += r;
	}
	return 1;
}

static int skip_bytes(size_t n)
{
	static uint8_t junk[65536];
	while (n) {
		size_t c = n > sizeof junk ? sizeof junk : n;
		if (!read_full(junk, c)) return 0;
		n -= c;
	}
	return 1;
}

static void *avi_reader(void *arg)
{
	(void)arg;
	uint8_t h[12];
	if (!read_full(h, 12) || memcmp(h, "RIFF", 4) || memcmp(h + 8, "AVI ", 4)) {
		fprintf(stderr, "plexfb: stdin is not an AVI stream\n");
		ring_eof = 1; return NULL;
	}
	size_t vneed = FRAME_BYTES;
	unsigned bad = 0;
	for (;;) {
		uint8_t ch[8];
		if (!read_full(ch, 8)) break;
		uint32_t sz = rd32(ch + 4);
		if (!memcmp(ch, "LIST", 4) || !memcmp(ch, "RIFF", 4)) {   /* descend into lists */
			uint8_t t[4];
			if (!read_full(t, 4)) break;
			continue;
		}
		size_t padded = sz + (sz & 1);
		if (!memcmp(ch, "strh", 4) && sz >= 32 && sz <= 256) {
			uint8_t b[256];
			if (!read_full(b, padded)) break;
			if (!memcmp(b, "vids", 4)) {
				uint32_t scale = rd32(b + 20), rate = rd32(b + 24);
				if (scale && rate) g_avi_fps = (double)rate / scale;
			}
			continue;
		}
		if (!memcmp(ch, "00dc", 4)) {
			if (sz != vneed) {
				if (sz && bad++ < 3) fprintf(stderr, "plexfb: video chunk %u bytes, expected %u\n", sz, (unsigned)vneed);
				if (!skip_bytes(padded)) break;
				continue;
			}
			while (ring_filled - ring_shown >= RING - 2) usleep(500);
			uint8_t *dst = ram_slot(ring_filled % RING);
			double a = now();
			if (!read_full(dst, sz)) break;
			if (sz & 1) skip_bytes(1);
			stat_read_ms += (now() - a) * 1e3;
			memcpy(map + BUF_OFF(ring_filled % RING), dst, sz);
			__sync_synchronize();
			ring_filled++;
		}
		else if (!memcmp(ch, "01wb", 4)) {
			size_t n = sz;
			while (n) {
				size_t off = a_w % AFIFO_BYTES;
				size_t c = n;
				if (c > AFIFO_BYTES - off) c = AFIFO_BYTES - off;
				if (c > 65536) c = 65536;
				while (AFIFO_BYTES - (a_w - a_r) < c) usleep(1000);
				if (!read_full(afifo + off, c)) goto done;
				__sync_synchronize();
				a_w += c;
				n -= c;
			}
			if (sz & 1) skip_bytes(1);
		}
		else if (!skip_bytes(padded)) break;      /* JUNK, ix00, idx1, ... */
	}
done:
	ring_eof = 1;
	return NULL;
}

static void *audio_thread(void *arg)
{
	(void)arg;
	int dbg = getenv("PLEXFB_DEBUG") != NULL;
	double tlast = now();
	for (;;) {
		double tv = (double)(ring_shown - ring_base) / g_fps;   /* picture time on screen */
		double ta = (double)a_r / ABPS;           /* audio time handed to aplay */
		if (dbg && now() - tlast > 1.0) {
			tlast = now();
			fprintf(stderr, "[a] tv=%.2f ta=%.2f queued=%.2fs filled=%u shown=%u eof=%d\n",
			        tv, ta, (a_w - a_r) / ABPS, ring_filled, ring_shown, ring_eof);
		}
		if (g_quit || (ring_eof && a_r >= a_w)) break;
		int drain = ring_eof && ring_filled == ring_shown;   /* picture over: let the tail out */
		if (a_w == a_r || (!drain && ta > tv + g_alead - g_aoff)) { usleep(2000); continue; }
		size_t c = a_w - a_r;
		if (c > 3840) c = 3840;                   /* 20 ms slices */
		size_t off = a_r % AFIFO_BYTES;
		if (c > AFIFO_BYTES - off) c = AFIFO_BYTES - off;
		if (fwrite(afifo + off, 1, c, aplay) != c) break;
		fflush(aplay);
		a_r += c;
	}
	return NULL;
}

int main(int argc, char **argv)
{
	const char *mode = argc > 1 ? argv[1] : "card";
	map_mem();

	if (!strcmp(mode, "status")) {
		for (int i = 0; i < 10; i++) {
			printf("magic=%08x seq=%u buf=%u %ux%u stride=%u | field_cnt=%u seq_shown=%u\n",
			       hdr->magic, hdr->seq, hdr->buf, hdr->width, hdr->height, hdr->stride,
			       stat->field_cnt, stat->seq_shown);
			usleep(200000);
		}
		return 0;
	}

	uint32_t *frame = malloc(W * H * 4);
	uint32_t seq = hdr->magic == MAGIC ? hdr->seq : 0;

	if (!strcmp(mode, "bench")) {       /* raw copy bandwidth into the mapping */
		draw_card(frame);
		double best = 1e9, tot = 0;
		for (int i = 0; i < 10; i++) {
			double a = now();
			copy_frame(i & 1, frame);
			double d = now() - a;
			tot += d; if (d < best) best = d;
		}
		printf("copy %u bytes: best %.1f ms (%.0f MB/s), avg %.1f ms (%.0f MB/s)\n",
		       W * H * 4, best * 1e3, W * H * 4 / best / 1e6, tot / 10 * 1e3, W * H * 4 / (tot / 10) / 1e6);
		return 0;
	}

	if (!strcmp(mode, "card")) {
		draw_card(frame);
		double t0 = now();
		copy_frame(0, frame);
		double t1 = now();
		publish(0, ++seq);
		printf("card published as seq %u (copy %.1f ms = %.0f MB/s)\n", seq,
		       (t1 - t0) * 1e3, W * H * 4 / (t1 - t0) / 1e6);
		return 0;
	}

	if (!strcmp(mode, "anim")) {
		double secs = argc > 2 ? atof(argv[2]) : 20;
		uint32_t last = stat->field_cnt;
		int buf = 0, t = 0, late = 0;
		double t0 = now(), tcopy = 0;
		while (now() - t0 < secs) {
			buf ^= 1;
			draw_anim(frame, t);
			double a = now();
			copy_frame(buf, frame);
			tcopy += now() - a;
			uint32_t before = stat->field_cnt;
			wait_field(&last);
			if (last - before > 1) late++;
			publish(buf, ++seq);
			t++;
		}
		printf("%d frames in %.1fs (%.1f/s), avg copy %.1f ms, %d late\n",
		       t, now() - t0, t / (now() - t0), tcopy / t * 1e3, late);
		return 0;
	}

	if (!strcmp(mode, "avi")) {
		g_fps = argc > 2 ? atof(argv[2]) : 23.976;
		g_yuv = 1;
		if (getenv("PLEXFB_ALEAD")) g_alead = atof(getenv("PLEXFB_ALEAD"));
		if (getenv("PLEXFB_AOFF"))  g_aoff  = atof(getenv("PLEXFB_AOFF"));
		if (fcntl(0, F_SETPIPE_SZ, 4 << 20) < 0) perror("F_SETPIPE_SZ (ignored)");
		signal(SIGUSR1, on_usr1);
		signal(SIGUSR2, on_usr2);
		signal(SIGTERM, on_term);
		signal(SIGINT, on_term);
		if (hdr->magic == MAGIC) ring_base = ring_filled = ring_shown = (hdr->buf + 1) % RING;
		afifo = malloc(AFIFO_BYTES);
		write_status(0, 0);            /* "starting": the UI stops drawing now */
		aplay = getenv("PLEXFB_MUTE") ? fopen("/dev/null", "w")
		      : popen("aplay -q -t raw -f S16_LE -c 2 -r 48000 --buffer-size=4800 -", "w");
		if (!aplay) { perror("aplay"); return 1; }
		pthread_t tr, ta;
		pthread_create(&tr, NULL, avi_reader, NULL);
		pthread_create(&ta, NULL, audio_thread, NULL);
		present_loop(g_fps, seq);
		pthread_join(ta, NULL);
		if (getenv("PLEXFB_MUTE")) fclose(aplay); else pclose(aplay);
		return 0;
	}

	if (!strcmp(mode, "raw")) {
		double fps = argc > 2 ? atof(argv[2]) : 29.97;
		g_yuv = argc > 3 && !strcmp(argv[3], "yuv");
		if (fcntl(0, F_SETPIPE_SZ, 4 << 20) < 0) perror("F_SETPIPE_SZ (ignored)");
		signal(SIGUSR1, on_usr1);
		signal(SIGTERM, on_term);
		signal(SIGINT, on_term);
		pthread_t th;
		pthread_create(&th, NULL, reader_thread, NULL);
		present_loop(fps, seq);
		return 0;
	}

	fprintf(stderr, "usage: plexfb card|anim [s]|raw [fps]|status\n");
	return 2;
}
