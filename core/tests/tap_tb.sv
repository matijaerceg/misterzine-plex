`timescale 1ns/1ps
module tap_tb;
reg clk=0,reset=1;
always #5 clk=~clk;
reg [31:0] seq=0;
wire signed [15:0] sample;
wire [15:0] measured_peak;
integer clocks=0,peak=0,value;
ui_tap dut(.clk(clk),.reset(reset),.sequence_in(seq),.sample(sample),.peak(measured_peak));
initial begin
 repeat(4) @(negedge clk);reset=0;
 repeat(1200) @(negedge clk);
 if(sample!=0)$fatal(1,"startup sound");
 seq=1;
 while(sample==0) begin @(negedge clk);clocks=clocks+1;end
 if(clocks>2250)$fatal(1,"late first sample: %d",clocks);
 $display("Tap starts within %d clocks of command (<42 us)",clocks);
 repeat(225000) begin
  @(negedge clk);
  value=sample<0?-sample:sample;
  if(value>peak)peak=value;
 end
 if(measured_peak!=peak)$fatal(1,"peak diagnostic mismatch");
 if(sample!=0 || peak>1700 || peak<1500)$fatal(1,"tap level/duration peak=%d",peak);
 $display("PASS quiet tap: peak=%d/32767, returns to silence, no replay",peak);
 $finish;
end
initial begin #5000000;$fatal(1,"timeout");end
endmodule
