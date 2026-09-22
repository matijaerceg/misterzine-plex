// A quiet, four-millisecond damped tap. Native core audio is mixed by the
// framework with ALSA, so navigation never queues behind theme PCM.
module ui_tap (
 input clk, input reset, input [31:0] sequence_in,
 output reg signed [15:0] sample = 0,
 output reg [15:0] peak = 0
);
reg [31:0] seen = 0;
reg initialized = 0;
reg [10:0] divider = 0;
reg [7:0] position = 0;
reg playing = 0;
function signed [15:0] wave(input [7:0] index);
 begin
  case(index)
   8'd0: wave = 16'sd0;
   8'd1: wave = 16'sd14;
   8'd2: wave = 16'sd51;
   8'd3: wave = 16'sd102;
   8'd4: wave = 16'sd160;
   8'd5: wave = 16'sd219;
   8'd6: wave = 16'sd275;
   8'd7: wave = 16'sd323;
   8'd8: wave = 16'sd361;
   8'd9: wave = 16'sd387;
   8'd10: wave = 16'sd400;
   8'd11: wave = 16'sd400;
   8'd12: wave = 16'sd387;
   8'd13: wave = 16'sd363;
   8'd14: wave = 16'sd328;
   8'd15: wave = 16'sd285;
   8'd16: wave = 16'sd234;
   8'd17: wave = 16'sd178;
   8'd18: wave = 16'sd119;
   8'd19: wave = 16'sd59;
   8'd20: wave = 16'sd0;
   8'd21: wave = -16'sd57;
   8'd22: wave = -16'sd110;
   8'd23: wave = -16'sd159;
   8'd24: wave = -16'sd201;
   8'd25: wave = -16'sd235;
   8'd26: wave = -16'sd263;
   8'd27: wave = -16'sd282;
   8'd28: wave = -16'sd294;
   8'd29: wave = -16'sd297;
   8'd30: wave = -16'sd293;
   8'd31: wave = -16'sd282;
   8'd32: wave = -16'sd264;
   8'd33: wave = -16'sd241;
   8'd34: wave = -16'sd213;
   8'd35: wave = -16'sd181;
   8'd36: wave = -16'sd146;
   8'd37: wave = -16'sd110;
   8'd38: wave = -16'sd73;
   8'd39: wave = -16'sd36;
   8'd40: wave = 16'sd0;
   8'd41: wave = 16'sd34;
   8'd42: wave = 16'sd65;
   8'd43: wave = 16'sd93;
   8'd44: wave = 16'sd117;
   8'd45: wave = 16'sd137;
   8'd46: wave = 16'sd152;
   8'd47: wave = 16'sd163;
   8'd48: wave = 16'sd169;
   8'd49: wave = 16'sd170;
   8'd50: wave = 16'sd168;
   8'd51: wave = 16'sd161;
   8'd52: wave = 16'sd151;
   8'd53: wave = 16'sd137;
   8'd54: wave = 16'sd121;
   8'd55: wave = 16'sd103;
   8'd56: wave = 16'sd83;
   8'd57: wave = 16'sd62;
   8'd58: wave = 16'sd41;
   8'd59: wave = 16'sd20;
   8'd60: wave = 16'sd0;
   8'd61: wave = -16'sd19;
   8'd62: wave = -16'sd37;
   8'd63: wave = -16'sd53;
   8'd64: wave = -16'sd66;
   8'd65: wave = -16'sd77;
   8'd66: wave = -16'sd86;
   8'd67: wave = -16'sd92;
   8'd68: wave = -16'sd95;
   8'd69: wave = -16'sd96;
   8'd70: wave = -16'sd95;
   8'd71: wave = -16'sd91;
   8'd72: wave = -16'sd85;
   8'd73: wave = -16'sd77;
   8'd74: wave = -16'sd68;
   8'd75: wave = -16'sd58;
   8'd76: wave = -16'sd47;
   8'd77: wave = -16'sd35;
   8'd78: wave = -16'sd23;
   8'd79: wave = -16'sd11;
   8'd80: wave = 16'sd0;
   8'd81: wave = 16'sd11;
   8'd82: wave = 16'sd21;
   8'd83: wave = 16'sd30;
   8'd84: wave = 16'sd37;
   8'd85: wave = 16'sd44;
   8'd86: wave = 16'sd49;
   8'd87: wave = 16'sd52;
   8'd88: wave = 16'sd54;
   8'd89: wave = 16'sd54;
   8'd90: wave = 16'sd53;
   8'd91: wave = 16'sd51;
   8'd92: wave = 16'sd48;
   8'd93: wave = 16'sd44;
   8'd94: wave = 16'sd39;
   8'd95: wave = 16'sd33;
   8'd96: wave = 16'sd26;
   8'd97: wave = 16'sd20;
   8'd98: wave = 16'sd13;
   8'd99: wave = 16'sd6;
   8'd100: wave = 16'sd0;
   8'd101: wave = -16'sd6;
   8'd102: wave = -16'sd12;
   8'd103: wave = -16'sd17;
   8'd104: wave = -16'sd21;
   8'd105: wave = -16'sd25;
   8'd106: wave = -16'sd27;
   8'd107: wave = -16'sd29;
   8'd108: wave = -16'sd30;
   8'd109: wave = -16'sd31;
   8'd110: wave = -16'sd30;
   8'd111: wave = -16'sd29;
   8'd112: wave = -16'sd27;
   8'd113: wave = -16'sd25;
   8'd114: wave = -16'sd22;
   8'd115: wave = -16'sd19;
   8'd116: wave = -16'sd15;
   8'd117: wave = -16'sd11;
   8'd118: wave = -16'sd7;
   8'd119: wave = -16'sd4;
   8'd120: wave = 16'sd0;
   8'd121: wave = 16'sd3;
   8'd122: wave = 16'sd7;
   8'd123: wave = 16'sd9;
   8'd124: wave = 16'sd12;
   8'd125: wave = 16'sd14;
   8'd126: wave = 16'sd15;
   8'd127: wave = 16'sd17;
   8'd128: wave = 16'sd17;
   8'd129: wave = 16'sd17;
   8'd130: wave = 16'sd17;
   8'd131: wave = 16'sd16;
   8'd132: wave = 16'sd15;
   8'd133: wave = 16'sd14;
   8'd134: wave = 16'sd12;
   8'd135: wave = 16'sd10;
   8'd136: wave = 16'sd8;
   8'd137: wave = 16'sd6;
   8'd138: wave = 16'sd4;
   8'd139: wave = 16'sd2;
   8'd140: wave = 16'sd0;
   8'd141: wave = -16'sd2;
   8'd142: wave = -16'sd4;
   8'd143: wave = -16'sd5;
   8'd144: wave = -16'sd7;
   8'd145: wave = -16'sd8;
   8'd146: wave = -16'sd9;
   8'd147: wave = -16'sd9;
   8'd148: wave = -16'sd10;
   8'd149: wave = -16'sd10;
   8'd150: wave = -16'sd10;
   8'd151: wave = -16'sd9;
   8'd152: wave = -16'sd9;
   8'd153: wave = -16'sd8;
   8'd154: wave = -16'sd7;
   8'd155: wave = -16'sd6;
   8'd156: wave = -16'sd5;
   8'd157: wave = -16'sd4;
   8'd158: wave = -16'sd2;
   8'd159: wave = -16'sd1;
   8'd160: wave = 16'sd0;
   8'd161: wave = 16'sd1;
   8'd162: wave = 16'sd2;
   8'd163: wave = 16'sd3;
   8'd164: wave = 16'sd4;
   8'd165: wave = 16'sd4;
   8'd166: wave = 16'sd5;
   8'd167: wave = 16'sd5;
   8'd168: wave = 16'sd5;
   8'd169: wave = 16'sd6;
   8'd170: wave = 16'sd5;
   8'd171: wave = 16'sd5;
   8'd172: wave = 16'sd5;
   8'd173: wave = 16'sd4;
   8'd174: wave = 16'sd4;
   8'd175: wave = 16'sd3;
   8'd176: wave = 16'sd3;
   8'd177: wave = 16'sd2;
   8'd178: wave = 16'sd1;
   8'd179: wave = 16'sd1;
   8'd180: wave = 16'sd0;
   8'd181: wave = -16'sd1;
   8'd182: wave = -16'sd1;
   8'd183: wave = -16'sd2;
   8'd184: wave = -16'sd2;
   8'd185: wave = -16'sd3;
   8'd186: wave = -16'sd3;
   8'd187: wave = -16'sd3;
   8'd188: wave = -16'sd3;
   8'd189: wave = -16'sd3;
   8'd190: wave = -16'sd3;
   8'd191: wave = -16'sd3;
   default: wave = 0;
  endcase
 end
endfunction
always @(posedge clk) begin
 if (reset) begin
  peak <= 0; sample <= 0; playing <= 0; initialized <= 0; divider <= 0;
 end else begin
  if ((sample[15] ? -sample : sample) > peak) peak <= sample[15] ? -sample : sample;
  if (!initialized) begin seen <= sequence_in; initialized <= 1; end
  else if (seen != sequence_in) begin
   seen <= sequence_in; position <= 0; playing <= 1;
  end
  if (divider == 11'd1124) begin
   divider <= 0;
   sample <= playing ? (wave(position) <<< 2) : 16'sd0;
   if (playing) begin
    if (position == 8'd191) playing <= 0;
    else position <= position + 8'd1;
   end
  end else divider <= divider + 11'd1;
 end
end
endmodule
