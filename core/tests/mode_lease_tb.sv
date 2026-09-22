`timescale 1ns/1ps
module mode_lease_tb;
reg clk=0,reset=1,vs=0;
always #5 clk=~clk;
wire [7:0] count;
wire [28:0] addr;
wire rd;
reg [63:0] data=0;
reg ready=0;
wire [1:0] mode;
wire fresh;
reg [31:0] seq_a=2,seq_b=2,request=32'h56500002;
integer remaining=0,offset=0,i;
ddr_scanout #(.BASE_WORDS(0)) dut(.clk(clk),.reset(reset),
 .DDRAM_BUSY(1'b0),.DDRAM_BURSTCNT(count),.DDRAM_ADDR(addr),
 .DDRAM_DOUT(data),.DDRAM_DOUT_READY(ready),.DDRAM_RD(rd),
 .DDRAM_DIN(),.DDRAM_BE(),.DDRAM_WE(),
 .ce_pix(1'b0),.hc(11'd1),.vc(10'd490),.next_row(10'd491),.active(1'b0),.vs(vs),
 .joy0(16'd0),.joy1(16'd0),.ps2_key(11'd0),.scan_status(4'd0),
 .app_mode(mode),.app_fresh(fresh),.valid(),.r(),.g(),.b(),.dbg_field_cnt());
always @(negedge clk) begin
 ready=0;
 if(rd) begin remaining=count; offset=addr; end
 if(remaining>0) begin
  ready=1;
  case(offset)
   14:data={request,seq_a};
   15:data={32'd0,seq_b};
   default:data=0;
  endcase
  offset=offset+1; remaining=remaining-1;
 end
end
task frame;
begin
 @(negedge clk);vs=1;
 repeat(100) @(negedge clk);
 vs=0;
 repeat(100) @(negedge clk);
end
endtask
initial begin
 repeat(3) @(negedge clk);reset=0;
 frame();
 if(!fresh || mode!=2) $fatal(1,"valid request rejected");
 seq_a=4;seq_b=2;request=32'h56500000;
 frame();
 if(mode!=2) $fatal(1,"torn request accepted");
 seq_b=4;
 frame();
 if(!fresh || mode!=0) $fatal(1,"480i request rejected");
 seq_a=6;seq_b=6;request=32'h56500003;
 frame();
 if(mode!=0) $fatal(1,"reserved mode accepted");
 for(i=0;i<122;i=i+1) frame();
 if(fresh) $fatal(1,"stale request did not expire");
 seq_a=8;seq_b=8;request=32'h56500002;
 frame();
 if(!fresh || mode!=2) $fatal(1,"new heartbeat did not recover");
 $display("PASS mode lease: atomicity, reserved mode rejection, expiry, recovery");
 $finish;
end
initial begin #10000000;$fatal(1,"timeout");end
endmodule
