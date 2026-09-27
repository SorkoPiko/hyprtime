#include <string.h>
#include <wayland-client.h>

#include "ext-idle-notify-v1-client-protocol.h"
#include "idle.h"

extern void goIdleEvent(int idle);

static struct wl_seat *seat;
static struct ext_idle_notifier_v1 *notifier;

static void registry_global(void *data, struct wl_registry *registry, uint32_t name,
                            const char *interface, uint32_t version) {
	if (strcmp(interface, wl_seat_interface.name) == 0 && seat == NULL) {
		seat = wl_registry_bind(registry, name, &wl_seat_interface, 1);
	} else if (strcmp(interface, ext_idle_notifier_v1_interface.name) == 0) {
		notifier = wl_registry_bind(registry, name, &ext_idle_notifier_v1_interface, 1);
	}
}

static void registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {}

static const struct wl_registry_listener registry_listener = {
	.global = registry_global,
	.global_remove = registry_global_remove,
};

static void notification_idled(void *data, struct ext_idle_notification_v1 *notification) {
	goIdleEvent(1);
}

static void notification_resumed(void *data, struct ext_idle_notification_v1 *notification) {
	goIdleEvent(0);
}

static const struct ext_idle_notification_v1_listener notification_listener = {
	.idled = notification_idled,
	.resumed = notification_resumed,
};

int idle_run(uint32_t timeout_ms) {
	struct wl_display *display = wl_display_connect(NULL);
	if (display == NULL) {
		return IDLE_ERR_CONNECT;
	}

	struct wl_registry *registry = wl_display_get_registry(display);
	wl_registry_add_listener(registry, &registry_listener, NULL);
	wl_display_roundtrip(display);

	if (seat == NULL || notifier == NULL) {
		wl_display_disconnect(display);
		return IDLE_ERR_UNSUPPORTED;
	}

	// get_idle_notification honours idle inhibitors, so playing video keeps
	// the session active the same way it keeps hypridle from firing
	struct ext_idle_notification_v1 *notification =
		ext_idle_notifier_v1_get_idle_notification(notifier, timeout_ms, seat);
	ext_idle_notification_v1_add_listener(notification, &notification_listener, NULL);

	while (wl_display_dispatch(display) != -1) {
	}

	wl_display_disconnect(display);
	return IDLE_ERR_DISCONNECTED;
}
