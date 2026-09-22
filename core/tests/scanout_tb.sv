`timescale 1ns/1ps
module scanout_tb;
reg clk=0,reset=1;
always #5 clk=~clk;
wire ce,vs,hb,vb;
wire [10:0] hc;
wire [9:0] vc,nr;
crt480i raster(.clk(clk),.reset(reset),.mode(2'd2),.vs_at_hs(1'b1),.band_grey(1'b0),
 .ce_pix(ce),.hs(),.vs(vs),.hblank(hb),.vblank(vb),.field(),.hc_o(hc),.vc_o(vc),.next_row(nr),.r(),.g(),.b());
wire [7:0] count;
wire [28:0] addr;
wire rd;
reg [63:0] data=0;
reg ready=0,busy=0;
wire [31:0] fields;
integer remaining=0,offset=0,latency=0,cycle=0;
ddr_scanout #(.BASE_WORDS(0)) dut(.clk(clk),.reset(reset),
 .DDRAM_BUSY(busy),.DDRAM_BURSTCNT(count),.DDRAM_ADDR(addr),
 .DDRAM_DOUT(data),.DDRAM_DOUT_READY(ready),.DDRAM_RD(rd),
 .DDRAM_DIN(),.DDRAM_BE(),.DDRAM_WE(),
 .ce_pix(ce),.hc(hc),.vc(vc),.next_row(nr),.active(!hb&&!vb),.vs(vs),
 .joy0(16'd0),.joy1(16'd0),.ps2_key(11'd0),.scan_status(4'd2),
 .app_mode(),.app_fresh(),.valid(),.r(),.g(),.b(),.dbg_field_cnt(fields));
always @(negedge clk) begin
 cycle=cycle+1;
 ready=0;
 // Delay every burst by 18 clocks; also stall one in 17 returned words.
 if(rd && !busy && remaining==0) begin remaining=count;offset=addr;latency=18;end
 busy=(remaining>0);
 if(latency>0) latency=latency-1;
 else if(remaining>0 && cycle%17!=0) begin
  ready=1;
  case(offset)
   0:data={32'd1,32'h504c4558};
   1:data={32'd720,32'd0};
   2:data={32'd720,32'd480};
   3:data={32'd43200,32'd1};
   4:data={32'd0,32'd54000};
   5:data={32'd0,32'd2};
   6:data={32'h680000,16'd480,16'd720};
   7:data={32'd2,32'd1};
   10,11,12,14,15:data=0;
   default:data=(offset>=29'hd0000)?64'h80ffffff80ffffff:64'h8080808080808080;
  endcase
  offset=offset+1;remaining=remaining-1;
 end
end
initial begin
 repeat(4) @(negedge clk);reset=0;
 wait(fields==3);
 repeat(100) @(negedge clk);
 if(dut.missed_lines!=0 || dut.max_fetch_clocks>=1716)
  $fatal(1,"fetch shortfall: max=%d missed=%d",dut.max_fetch_clocks,dut.missed_lines);
 if(dut.max_fetch_clocks<600) $fatal(1,"full overlay fetch not exercised");
 $display("PASS 480p YUV + full overlay: max=%d/1716 clocks, missed=%d",dut.max_fetch_clocks,dut.missed_lines);
 $finish;
end
initial begin #40000000;$fatal(1,"timeout");end
endmodule
