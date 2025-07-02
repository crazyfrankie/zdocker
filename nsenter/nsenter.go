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
#include <sys/wait.h>
#include <sys/mount.h>
#include <sys/syscall.h>
#include <signal.h>

#define ZDOCKER_INIT_ENV "ZDOCKER_INIT"

// clone flags for container creation
#define CLONE_FLAGS (CLONE_NEWUTS | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET | CLONE_NEWIPC)

// The attribute ((constructor)) here means that the function will be executed automatically once the package is referenced.
// This runs BEFORE Go runtime starts
__attribute__((constructor)) void zdocker_init(void) {
	char *zdocker_init;
	zdocker_init = getenv(ZDOCKER_INIT_ENV);

	if (zdocker_init && strcmp(zdocker_init, "1") == 0) {
		// This is the container init process
		// The Go runtime will handle the rest
		return;
	}

	// Check if this is an exec into existing container
	char *zdocker_pid;
	zdocker_pid = getenv("zdocker_pid");
	if (zdocker_pid) {
		// This is exec into existing container - use original logic
		char *zdocker_cmd;
		zdocker_cmd = getenv("zdocker_cmd");
		if (!zdocker_cmd) {
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

	// Check if this should create a new container
	char *zdocker_create;
	zdocker_create = getenv("ZDOCKER_CREATE");
	if (!zdocker_create || strcmp(zdocker_create, "1") != 0) {
		// Not a container creation, continue normal execution
		return;
	}

	// This is container creation - we need to clone with namespaces
	fprintf(stdout, "zdocker: creating container with namespaces\n");

	// Create pipe for communication
	int pipefd[2];
	if (pipe(pipefd) == -1) {
		fprintf(stderr, "zdocker: failed to create pipe: %s\n", strerror(errno));
		exit(1);
	}

	// Clone with namespaces using proper syscall
	pid_t child_pid = syscall(SYS_clone, CLONE_FLAGS | SIGCHLD, NULL, NULL, NULL, NULL);

	if (child_pid == -1) {
		fprintf(stderr, "zdocker: clone failed: %s\n", strerror(errno));
		exit(1);
	}

	if (child_pid == 0) {
		// Child process - this will become the container init
		close(pipefd[0]); // Close read end

		// Set environment variable to indicate this is container init
		if (setenv(ZDOCKER_INIT_ENV, "1", 1) != 0) {
			fprintf(stderr, "zdocker: failed to set init env: %s\n", strerror(errno));
			exit(1);
		}

		// Write child PID to pipe (for parent to read)
		pid_t my_pid = getpid();
		if (write(pipefd[1], &my_pid, sizeof(my_pid)) != sizeof(my_pid)) {
			fprintf(stderr, "zdocker: failed to write pid to pipe: %s\n", strerror(errno));
			exit(1);
		}
		close(pipefd[1]);

		// Continue to Go runtime - it will handle container initialization
		return;
	}

	// Parent process
	close(pipefd[1]); // Close write end

	// Read child PID
	pid_t container_pid;
	if (read(pipefd[0], &container_pid, sizeof(container_pid)) != sizeof(container_pid)) {
		fprintf(stderr, "zdocker: failed to read container pid: %s\n", strerror(errno));
		exit(1);
	}
	close(pipefd[0]);

	// Wait for child and exit
	int status;
	waitpid(child_pid, &status, 0);
	exit(WEXITSTATUS(status));
}
*/
import "C"
