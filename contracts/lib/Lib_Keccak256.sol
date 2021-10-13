// SPDX-License-Identifier: MIT
pragma solidity >0.5.0 <0.8.0;

// https://chenglongma.com/10/simple-keccak/
// https://github.com/firefly/wallet/blob/master/source/libs/ethers/src/keccak256.c

library Lib_Keccak256 {
  struct CTX {
    uint64[25] A;
  }

  bytes public constant round_constant = hex"011a5e701f2179550e0c35263f4f5d535248166679582174";

  function ROTL64(uint64 qword, uint64 n) internal pure returns (uint64) {
    return ((qword) << (n) ^ ((qword) >> (64 - (n))));
  }

  function get_round_constant(uint round) internal pure returns (uint64) {
    uint64 result = 0;
    uint8 roundInfo = uint8(round_constant[round]);
    if (roundInfo & (1 << 6) != 0) { result |= (1 << 63); }
    if (roundInfo & (1 << 5) != 0) { result |= (1 << 31); }
    if (roundInfo & (1 << 4) != 0) { result |= (1 << 15); }
    if (roundInfo & (1 << 3) != 0) { result |= (1 << 7); }
    if (roundInfo & (1 << 2) != 0) { result |= (1 << 3); }
    if (roundInfo & (1 << 1) != 0) { result |= (1 << 1); }
    if (roundInfo & (1 << 0) != 0) { result |= (1 << 0); }
    return result;
  }

  function keccak_theta(CTX memory c) internal pure {
    uint64[5] memory C;
    uint64[5] memory D;
    uint i;
    uint j;
    for (i = 0; i < 5; i++) {
      C[i] = c.A[i];
      for (j = 5; j < 25; j += 5) { C[i] ^= c.A[i + j]; }
    }
    for (i = 0; i < 5; i++) {
      D[i] = ROTL64(C[(i + 1) % 5], 1) ^ C[(i + 4) % 5];
    }
    for (i = 0; i < 5; i++) {
      for (j = 0; j < 25; j += 5) { c.A[i + j] ^= D[i]; }
    }
  }

  function keccak_rho(CTX memory c) internal pure {
    c.A[1] = ROTL64(c.A[1], 1);
    c.A[2] = ROTL64(c.A[2], 62);
    c.A[3] = ROTL64(c.A[3], 28);
    c.A[4] = ROTL64(c.A[4], 27);
    c.A[5] = ROTL64(c.A[5], 36);
    c.A[6] = ROTL64(c.A[6], 44);
    c.A[7] = ROTL64(c.A[7], 6);
    c.A[8] = ROTL64(c.A[8], 55);
    c.A[9] = ROTL64(c.A[9], 20);
    c.A[10] = ROTL64(c.A[10], 3);
    c.A[11] = ROTL64(c.A[11], 10);
    c.A[12] = ROTL64(c.A[12], 43);
    c.A[13] = ROTL64(c.A[13], 25);
    c.A[14] = ROTL64(c.A[14], 39);
    c.A[15] = ROTL64(c.A[15], 41);
    c.A[16] = ROTL64(c.A[16], 45);
    c.A[17] = ROTL64(c.A[17], 15);
    c.A[18] = ROTL64(c.A[18], 21);
    c.A[19] = ROTL64(c.A[19], 8);
    c.A[20] = ROTL64(c.A[20], 18);
    c.A[21] = ROTL64(c.A[21], 2);
    c.A[22] = ROTL64(c.A[22], 61);
    c.A[23] = ROTL64(c.A[23], 56);
    c.A[24] = ROTL64(c.A[24], 14);
  }

  function keccak_pi(CTX memory c) internal pure {
    uint64 A1 = c.A[1];
    c.A[1] = c.A[6];
    c.A[6] = c.A[9];
    c.A[9] = c.A[22];
    c.A[22] = c.A[14];
    c.A[14] = c.A[20];
    c.A[20] = c.A[2];
    c.A[2] = c.A[12];
    c.A[12] = c.A[13];
    c.A[13] = c.A[19];
    c.A[19] = c.A[23];
    c.A[23] = c.A[15];
    c.A[15] = c.A[4];
    c.A[4] = c.A[24];
    c.A[24] = c.A[21];
    c.A[21] = c.A[8];
    c.A[8] = c.A[16];
    c.A[16] = c.A[5];
    c.A[5] = c.A[3];
    c.A[3] = c.A[18];
    c.A[18] = c.A[17];
    c.A[17] = c.A[11];
    c.A[11] = c.A[7];
    c.A[7] = c.A[10];
    c.A[10] = A1;
  }

  function keccak_chi(CTX memory c) internal pure {
    uint i;
    uint64 A0;
    uint64 A1;
    for (i = 0; i < 25; i+=5) {
      A0 = c.A[0 + i];
      A1 = c.A[1 + i];
      c.A[0 + i] ^= ~A1 & c.A[2 + i];
      c.A[1 + i] ^= ~c.A[2 + i] & c.A[3 + i];
      c.A[2 + i] ^= ~c.A[3 + i] & c.A[4 + i];
      c.A[3 + i] ^= ~c.A[4 + i] & A0;
      c.A[4 + i] ^= ~A0 & A1;
    }
  }

  function keccak_init(CTX memory c) internal pure {
    // is this needed?
    uint i;
    for (i = 0; i < 25; i++) {
      c.A[i] = 0;
    }
  }

  function sha3_permutation(CTX memory c) internal pure {
    uint round;
    for (round = 0; round < 24; round++) {
      keccak_theta(c);
      keccak_rho(c);
      keccak_pi(c);
      keccak_chi(c);
      // keccak_iota
      c.A[0] ^= get_round_constant(round);
    }
  }

  // https://stackoverflow.com/questions/2182002/convert-big-endian-to-little-endian-in-c-without-using-provided-func
  function flip(uint64 val) internal pure returns (uint64) {
    val = ((val << 8) & 0xFF00FF00FF00FF00 ) | ((val >> 8) & 0x00FF00FF00FF00FF );
    val = ((val << 16) & 0xFFFF0000FFFF0000 ) | ((val >> 16) & 0x0000FFFF0000FFFF );
    return (val << 32) | (val >> 32);
  }

  function get_hash(CTX memory c) internal pure returns (bytes32) {
    return bytes32((uint256(flip(c.A[0])) << 192) |
                   (uint256(flip(c.A[1])) << 128) |
                   (uint256(flip(c.A[2])) << 64) |
                   (uint256(flip(c.A[3])) << 0));
  }

}