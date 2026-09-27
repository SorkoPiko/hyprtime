#ifndef HYPRTIME_IDLE_H
#define HYPRTIME_IDLE_H

#include <stdint.h>

#define IDLE_ERR_CONNECT 1
#define IDLE_ERR_UNSUPPORTED 2
#define IDLE_ERR_DISCONNECTED 3

int idle_run(uint32_t timeout_ms);

#endif
