`timescale 1ns/1ps
module hdmi_scale_tb;
reg clk=0;
always #5 clk=~clk;
reg [11:0] width=1920, height=1080;
reg [2:0] mode=0;
wire [12:0] x,y;
video_scale_int dut(clk,width,height,mode,12'd720,12'd480,12'd4,12'd3,x,y);
task check(input [12:0] expected_x, input [12:0] expected_y);
 begin
  repeat(4000) @(negedge clk);
  if(x!==expected_x || y!==expected_y)
   $fatal(1,"mode=%d output=%dx%d got=%d,%d expected=%d,%d",mode,width,height,x,y,expected_x,expected_y);
 end
endtask
initial begin
 check(4,3);
 mode=1; check(4096+1280,4096+960);
 width=1280;height=720;check(4096+640,4096+480);
 width=1920;height=1440;check(4096+1920,4096+1440);
 width=640;height=480;check(4096+640,4096+480);
 width=720;check(4096+640,4096+480);
 mode=0;check(4,3);
 mode=1;height=240;check(4,3);
 $display("PASS HDMI scale: 1080p, 720p, 1440p, 480p, mode changes and undersized output");
 $finish;
end
endmodule
