//============================================================================
//  crt480i - NTSC 480i / 240p timing generator + test pattern
//
//  M1 bring-up core for the MiSTer Plex client. Purpose: prove a custom core
//  can put a genuine 480-line interlaced raster out of the analog port with
//  the framework Y/C encoder active.
//
//  Pattern is generated procedurally from the pixel counters - no DDR, no
//  host, nothing to go wrong except the timing itself. DDR frame fetch is M2.
//
//  NTSC 480i:  13.5 MHz pixel clock, 858 x 525, Fh = 15734.27 Hz, Fv = 59.94
//  NTSC 240p:  13.5 MHz pixel clock, 858 x 262, Fh = 15734.27 Hz, Fv = 60.05
//  clk is 54 MHz, ce_pix is 1-in-4.
//
//  Interlace is real: the vertical counter steps in FRAME lines (2 at a time,
//  opposite parity each field) and the vsync transition is evaluated half a
//  line later on the odd field. That half-line offset is what makes a CRT
//  interleave the fields instead of painting them on top of each other.
//  In 240p every frame is 262 whole lines and vsync sits at the same
//  horizontal phase every frame, so the CRT paints every line in the same
//  place and shows the usual gaps between scanlines.
//============================================================================

module crt480i
(
	input             clk,        // 54 MHz
	input             reset,

	input       [1:0] mode,       // 0 = 480i, 1 = 240p, 2 = 480p
	input             vs_at_hs,   // 1 = vsync transitions at the hsync leading edge (textbook)
	                              // 0 = vsync transitions at the start of active video (builds 1-3)
	input             band_grey,  // 1 = replace the flicker band with flat grey (field-symmetric load)

	output            ce_pix,     // 13.5 MHz clock enable
	output reg        hs,
	output reg        vs,
	output            hblank,
	output            vblank,
	output reg        field,      // -> VGA_F1
	output     [10:0] hc_o,
	output      [9:0] vc_o,
	output      [9:0] next_row,

	output reg [7:0]  r,
	output reg [7:0]  g,
	output reg [7:0]  b
);

// ---------------- timing constants ----------------
// BT.601 525-line: 858 px per line at 13.5 MHz, 720 px active, 16 px front
// porch, 64 px (4.7 us) sync, 58 px back porch. 720 active is what DVD-class
// video is mastered to, so the frame buffer maps 1:1 with no scaling.
localparam HACT = 11'd720;
localparam HFP  = 11'd16;
localparam HSW  = 11'd64;
localparam HTOT = 11'd858;     // back porch = 858 - 720 - 16 - 64 = 58

localparam VACT_I = 10'd480;   // interlaced: frame lines
localparam VTOT_I = 10'd525;
localparam VACT_P = 10'd240;   // progressive 240p
localparam VTOT_P = 10'd262;

localparam VFP = 10'd3;
localparam VSW = 10'd6;

wire [10:0] HS_START = HACT + HFP;          // 736
wire interlace = (mode == 2'd0);
wire lowres = (mode == 2'd1);
wire progressive480 = (mode == 2'd2);
// CTA 480p uses 62 sync pixels and a 9-line vertical front porch.
wire [10:0] HS_END   = HACT + HFP + (progressive480 ? 11'd62 : HSW);

wire [9:0] VACT = lowres ? VACT_P : VACT_I;
wire [9:0] VTOT = lowres ? VTOT_P : VTOT_I;
wire [9:0] VS_START = VACT + (progressive480 ? 10'd9 : VFP);
wire [9:0] VS_END   = VS_START + VSW;

// ---------------- pixel clock enable: 54 / 4 = 13.5 ----------------
reg [1:0] cediv = 2'd0;
assign ce_pix = (cediv == 2'd0);
always @(posedge clk) begin
	if (reset || mode_chg) cediv <= 2'd0;
	else if (progressive480) cediv <= {1'b0, ~cediv[0]};
	else cediv <= cediv + 2'd1;
end

// ---------------- counters ----------------
reg [10:0] hc = 11'd0;
reg  [9:0] vc = 10'd0;

// last line of a field: interlaced fields end at VTOT-1 (even) / VTOT-2 (odd)
wire vc_last = interlace ? (vc >= (VTOT - 10'd2)) : (vc >= (VTOT - 10'd1));

// Restart the raster cleanly when the mode is switched, otherwise the counters
// can be left mid-frame holding values that are out of range for the new mode.
reg [1:0] mode_d = 2'd0;
wire mode_chg = (mode != mode_d);
always @(posedge clk) begin
	mode_d <= mode;
end

always @(posedge clk) begin
	if (reset || mode_chg) begin
		hc    <= 11'd0;
		vc    <= 10'd0;
		field <= 1'b0;
	end
	else if (ce_pix) begin
		if (hc == (HTOT - 11'd1)) begin
			hc <= 11'd0;
			if (vc_last) begin
				if (interlace) begin
					// next field starts on the opposite parity
					vc    <= field ? 10'd0 : 10'd1;
					field <= ~field;
				end
				else begin
					vc    <= 10'd0;
					field <= 1'b0;
				end
			end
			else vc <= interlace ? (vc + 10'd2) : (vc + 10'd1);
		end
		else hc <= hc + 11'd1;
	end
end

// ---------------- sync ----------------
// Where in the line the vsync register is (re)evaluated.
//
// Even field / progressive:  vs_base  = HS_START (textbook) or 0 (legacy)
// Odd field:                 half a line later.  HS_START + 429 = 1165, which
//                            is column 307 of the NEXT line, so in textbook
//                            mode the odd field also compares against vc-2
//                            to land on the right line.
//
// Either way the even->odd and odd->even vsync intervals are exactly 262.5
// lines in 480i, and exactly 262 lines with a fixed phase in 240p.
wire [10:0] vs_base = (vs_at_hs || progressive480) ? HS_START : 11'd0;
wire [10:0] vs_odd  = vs_at_hs ? (HS_START + (HTOT >> 1) - HTOT)   // 307
                               : (HTOT >> 1);                      // 429
wire [10:0] vs_eval = (interlace && field) ? vs_odd : vs_base;
wire  [9:0] vc_ref  = (interlace && field && vs_at_hs) ? (vc - 10'd2) : vc;

always @(posedge clk) begin
	if (reset || mode_chg) begin
		hs <= 1'b0;
		vs <= 1'b0;
	end else if (ce_pix) begin
		hs <= (hc >= HS_START) && (hc < HS_END);
		if (hc == vs_eval) vs <= (vc_ref >= VS_START) && (vc_ref < VS_END);
	end
end

assign hblank = (hc >= HACT);
assign vblank = (vc >= VACT);

// exported for the DDR scan-out: where we are, and which frame row the next
// line will show (so it can be fetched a line ahead)
assign hc_o = hc;
assign vc_o = vc;
assign next_row = vc_last ? (interlace ? (field ? 10'd0 : 10'd1) : 10'd0)
                          : (interlace ? (vc + 10'd2) : (vc + 10'd1));

// ---------------- test pattern ----------------
// py : vertical coordinate in FRAME rows (0..479 interlaced, 0..239 progressive)
// sy : the same position in SCREEN units, always 0..479, so every element of
//      the pattern has the same size and place on the tube in both modes.
wire  [9:0] py = vc;
wire  [9:0] sy = lowres ? {py[8:0], 1'b0} : py;
wire [10:0] sx = hc;

// 8 colour bars across 720 px (90 px each)
wire [2:0] bar = (hc < 11'd90)  ? 3'd0 :
                 (hc < 11'd180) ? 3'd1 :
                 (hc < 11'd270) ? 3'd2 :
                 (hc < 11'd360) ? 3'd3 :
                 (hc < 11'd450) ? 3'd4 :
                 (hc < 11'd540) ? 3'd5 :
                 (hc < 11'd630) ? 3'd6 : 3'd7;

reg [23:0] barcol;
always @(*) begin
	case (bar)
		3'd0: barcol = 24'hEBEBEB;  // white
		3'd1: barcol = 24'hEBEB10;  // yellow
		3'd2: barcol = 24'h10EBEB;  // cyan
		3'd3: barcol = 24'h10EB10;  // green
		3'd4: barcol = 24'hEB10EB;  // magenta
		3'd5: barcol = 24'hEB1010;  // red
		3'd6: barcol = 24'h1010EB;  // blue
		default: barcol = 24'h101010;
	endcase
end

// grey ramp, 0..255 over the first 680 px (hc/4 * 1.5), white after
wire [7:0] ramp = (hc >= 11'd680) ? 8'd255 : (hc[9:2] + hc[9:3]);

// ---- ring: centre (360, 300 screen rows), radius 56 px, ~1.5 px thick ----
// Pipelined over 3 clocks (the counters only move every 4th clock, so the
// result is at most one pixel late, which does not matter for a test card).
reg signed [11:0] rdx, rdy;
reg signed [23:0] rdx2, rdy2;
reg        [24:0] rsum;
reg               ring;
always @(posedge clk) begin
	rdx  <= $signed({1'b0, sx}) - 12'sd360;
	rdy  <= $signed({2'b0, sy}) - 12'sd300;
	rdx2 <= rdx * rdx;
	rdy2 <= rdy * rdy;
	rsum <= rdx2[23:0] + rdy2[23:0];
	// 56^2 = 3136; band of +-170 around it is about 1.5 px wide
	ring <= (rsum > 25'd2966) && (rsum < 25'd3306);
end

// ---- big X across the geometry band: two 45-degree lines, 3 px wide ----
wire [10:0] xa = {1'b0, sy} + 11'd60;          // sy 240..359 -> x 300..419
wire [10:0] xb = 11'd660 - {1'b0, sy};         // sy 240..359 -> x 420..301
wire xline = ((sx + 11'd1 >= xa) && (sx <= xa + 11'd1)) ||
             ((sx + 11'd1 >= xb) && (sx <= xb + 11'd1));

// ---- border, 4 screen rows / 8 px: red = 240p, green = 480i ----
wire border = (sx < 11'd8) || (sx >= (HACT - 11'd8)) || (sy < 10'd4) || (sy >= 10'd476);

always @(posedge clk) begin
	if (ce_pix) begin
		if (hblank || vblank) begin
			{r, g, b} <= 24'h000000;
		end
		else if (border) begin
			{r, g, b} <= interlace ? 24'h00FF00 : 24'hFF0000;
		end
		// BAND 1 (screen rows 0..119): FLICKER TEST.
		// Alternating single FRAME rows, white/black.
		//   480i : white rows are all in one field, black rows all in the
		//          other -> the whole band flashes white/black at 30 Hz.
		//   240p : stable alternating scanlines, no flicker at all.
		// The flicker band puts all its light in ONE field. On a consumer set
		// that pulls the EHT down during the bright field and stretches its
		// raster for the next few dozen lines, which looks like a field
		// offset that varies down the screen. band_grey swaps in a flat grey
		// of the same average brightness so both fields load the set equally.
		else if (sy < 10'd120) begin
			{r, g, b} <= band_grey ? 24'h7E7E7E : (py[0] ? 24'hEBEBEB : 24'h101010);
		end
		// BAND 2 (120..239): solid colour bars - look for the gaps
		// between scanlines that only 240p has.
		else if (sy < 10'd240) begin
			{r, g, b} <= barcol;
		end
		// BAND 3 (240..359): geometry. A ring and an X.
		//   240p : visibly stepped, every step a whole scanline apart.
		//   480i : twice as fine, near-horizontal parts of the ring shimmer.
		else if (sy < 10'd360) begin
			{r, g, b} <= ring  ? 24'h10EBEB :
			             xline ? 24'hEBEB10 : 24'h101010;
		end
		// BAND 4 (360..479): grey ramp.
		else begin
			{r, g, b} <= {ramp, ramp, ramp};
		end
	end
end

endmodule
