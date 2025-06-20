package nsenter

/*
#define _GNU_SOURCE
#include <unistd.h>
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>

// The attribute ((constructor)) here means that the function will be executed automatically once the package is referenced.
// Similar to constructors, which are run at program startup
__attribute__((constructor)) void enter_namespace(void) {
	char *zdocker_pid;
	zdocker_pid = getenv("zdocker_pid");
	if (zdocker_pid) {
		// fprintf(stdout, "got zdocker_pid=%s\n", zdocker_pid);
	} else {
		// fprintf(stdout, "missing zdocker_pid env skip nsenter");
		// Here, if PIO is not specified, there is no need to execute downward, and exit directly
		return;
	}
	char *zdocker_cmd;
	zdocker_cmd = getenv("zdocker_cmd");
	if (zdocker_cmd) {
		// fprintf(stdout, "got zdocker_cmd=%s\n", zdocker_cmd);
	} else {
		// fprintf(stdout, "missing zdocker_cmd env skip nsenter");
		return;
	}
	int i;
	char nspath[1024];
	char *namespaces[] = { "ipc", "uts", "net", "pid", "mnt" };

	for (i=0; i<5; i++) {
		sprintf(nspath, "/proc/%s/ns/%s", zdocker_pid, namespaces[i]);
		int fd = open(nspath, O_RDONLY);

		if (setns(fd, 0) == -1) {
			// fprintf(stderr, "setns on %s namespace failed: %s\n", namespaces[i], strerror(errno));
		} else {
			// fprintf(stdout, "setns on %s namespace succeeded\n", namespaces[i]);
		}
		close(fd);
	}
	int res = system(zdocker_cmd);
	exit(0);
	return;
}
*/
import "C"
