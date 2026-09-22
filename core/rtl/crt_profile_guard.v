// Read the same active-profile commands as the MiSTer framework. Y/C and
// component profiles on this CRT client always use the native 15 kHz raster.
// Stay safe until both configuration words have arrived after core loading.
module crt_profile_guard (
 input clk,
 input io_enable, io_strobe,
 input [15:0] io_din,
 input [1:0] requested_mode,
 output locked,
 output [1:0] safe_mode
);
reg cfg_seen=0, yc_seen=0, component=0, yc=0;
reg [15:0] command=0;
reg [1:0] word_count=0;
assign locked = !cfg_seen || !yc_seen || component || yc;
assign safe_mode = locked ? 2'd0 : requested_mode;
always @(posedge clk) begin
 if (!io_enable) word_count <= 0;
 else if (io_strobe) begin
  if (word_count == 0) begin command <= io_din; word_count <= 1; end
  else begin
   word_count <= 2;
   if (word_count == 1) begin
    if (command == 16'h0001) begin
     component <= io_din[5]; cfg_seen <= 1;
    end
    if (command == 16'h0041) begin
     yc <= io_din[0]; yc_seen <= 1;
    end
   end
  end
 end
end
endmodule
