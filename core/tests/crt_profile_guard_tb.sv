`timescale 1ns/1ps
module crt_profile_guard_tb;
reg clk=0;
always #5 clk=~clk;
reg enable=0,strobe=0;
reg [15:0] data=0;
reg [1:0] requested=2;
wire locked;
wire [1:0] mode;
crt_profile_guard dut(clk,enable,strobe,data,requested,locked,mode);
task word(input [15:0] value);
 begin @(negedge clk);data=value;strobe=1;@(negedge clk);strobe=0;end
endtask
task command(input [15:0] cmd,input [15:0] value);
 begin @(negedge clk);enable=1;word(cmd);word(value);@(negedge clk);enable=0;@(negedge clk);end
endtask
task check(input expected);
 begin #1;if(locked!==expected || mode!==(expected?2'd0:requested))$fatal(1,"guard locked=%b mode=%d",locked,mode);end
endtask
initial begin
 check(1); // saved/forced 480p cannot escape before profile initialization
 command(1,0);check(1);
 command('h41,0);check(0); // HDMI profile
 command('h41,1);check(1); // S-video
 command(1,3);check(1); // buttons must not clear Y/C lock
 requested=1;check(1); // even forced 240p becomes the defined safe 480i
 requested=2;
 command('h41,0);check(0);
 command(1,32);check(1); // component profile
 command(1,0);check(0); // return to HDMI restores requested preference
 @(negedge clk);enable=1;word('h41);word(0);word(1);word(1);
 @(negedge clk);enable=0;@(negedge clk);check(0); // phase words aren't mode flags
 command('h41,3);check(1); // composite
 command('h41,0);check(0);
 $display("PASS CRT profile guard: startup, saved/forced modes, Y/C, component, live changes, payload framing");
 $finish;
end
endmodule
