#pragma once

#define WINAPIGEN_HEADER_CONSTANT 0x20u
#define WINAPIGEN_HEADER_FUNCTION(x, y) ((x) + (y))

typedef struct WINAPIGEN_HEADER_BITS {
    unsigned int enabled : 1;
    signed int delta : 5;
} WINAPIGEN_HEADER_BITS;

static inline unsigned int winapigen_header_inline(unsigned int value) {
    return value + WINAPIGEN_HEADER_CONSTANT;
}
