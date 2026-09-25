`timescale 1ns/1ps
module brightness_tb;
reg clk=0,reset=1,vs=0;
always #5 clk=~clk;
wire [7:0] count;
wire [28:0] addr;
wire rd,we;
wire [63:0] din;
reg [63:0] data=0;
reg ready=0;
reg [31:0] word=0,echo=0;
integer remaining=0,offset=0;
ddr_scanout #(.BASE_WORDS(0)) dut(.clk(clk),.reset(reset),
 .DDRAM_BUSY(1'b0),.DDRAM_BURSTCNT(count),.DDRAM_ADDR(addr),
 .DDRAM_DOUT(data),.DDRAM_DOUT_READY(ready),.DDRAM_RD(rd),
 .DDRAM_DIN(din),.DDRAM_BE(),.DDRAM_WE(we),
 .ce_pix(1'b0),.hc(11'd1),.vc(10'd490),.next_row(10'd491),.active(1'b0),.vs(vs),
 .joy0(16'd0),.joy1(16'd0),.ps2_key(11'd0),.scan_status(4'd0),
 .app_mode(),.app_fresh(),.valid(),.r(),.g(),.b(),.dbg_field_cnt());
always @(negedge clk) begin
 ready=0;
 if(we && addr==13) echo=din[31:0];
 if(rd) begin remaining=count; offset=addr; end
 if(remaining>0) begin
  ready=1;
  data=(offset==17)?{32'd0,word}:64'd0;   // +0x88
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
task check(input [31:0] w, input [8:0] want);
begin
 word=w; frame();
 if(dut.lvl!==want) $fatal(1,"word %h: level %0d, want %0d",w,dut.lvl,want);
 if(echo[24:16]!==want || echo[9:0]!==dut.d_x) $fatal(1,"echo %h for level %0d",echo,want);
end
endtask
initial begin
 repeat(3) @(negedge clk);reset=0;
 check(32'h00000000,256);   // a wiped ring
 check(32'h444D0040,64);
 check(32'h444D0000,0);
 check(32'h444D0101,256);   // out of range
 check(32'h124D0040,256);   // not the tag
 check(32'h444D0100,256);
 $display("PASS brightness word: tag, range, echo");
 $finish;
end
initial begin #10000000;$fatal(1,"timeout");end
endmodule
