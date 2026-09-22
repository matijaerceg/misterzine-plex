`timescale 1ns/1ps
module raster_tb;
reg clk=0, reset=1;
always #5 clk=~clk;
reg [1:0] mode=0;
wire ce,hs,vs,hb,vb,field;
wire [10:0] hc;
wire [9:0] vc,next_row;
crt480i dut(.clk(clk),.reset(reset),.mode(mode),.vs_at_hs(1'b1),.band_grey(1'b0),
 .ce_pix(ce),.hs(hs),.vs(vs),.hblank(hb),.vblank(vb),.field(field),
 .hc_o(hc),.vc_o(vc),.next_row(next_row),.r(),.g(),.b());
integer clocks, pixels, lines, period, i;
task check_progressive(input [1:0] m, input integer height, total, divider);
begin
 @(negedge clk); mode=m; reset=1;
 repeat(4) @(negedge clk);
 reset=0;
 // Start at a complete frame boundary after the reset transient.
 @(negedge clk);
 while (!(ce && hc==0 && vc==0)) @(negedge clk);
 clocks=0; pixels=0; lines=0;
 begin: frame_loop
 forever begin
  if (field) $fatal(1,"progressive field flag");
  if (ce) begin
   if (!hb && !vb) pixels=pixels+1;
   if (hc==0) lines=lines+1;
   if (next_row != ((vc==total-1)?0:vc+1)) $fatal(1,"next row");
  end
  clocks=clocks+1;
  @(negedge clk);
  if (ce && hc==0 && vc==0) disable frame_loop;
 end
 end
 if (clocks != 858*total*divider || pixels != 720*height || lines != total)
  $fatal(1,"mode %d: clocks=%d pixels=%d lines=%d",m,clocks,pixels,lines);
 $display("PASS mode %d: %d active pixels, %d clocks/frame",m,pixels,clocks);
end
endtask
initial begin
 check_progressive(1,240,262,4);
 check_progressive(2,480,525,2);
 @(negedge clk); mode=0;
 // Existing interlaced field intervals must remain exactly 262.5 lines.
 @(posedge vs); @(negedge clk);
 for(i=0;i<3;i=i+1) begin
  clocks=0;
  while(vs) begin @(negedge clk); clocks=clocks+1; end
  while(!vs) begin @(negedge clk); clocks=clocks+1; end
  if(clocks!=900900) $fatal(1,"480i field interval %d",clocks);
 end
 $display("PASS 480i: 900900 clocks/field after mode switch");
 $finish;
end
initial begin #200000000; $fatal(1,"timeout"); end
endmodule
