#include <errno.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <time.h>
#include <unistd.h>

int nanosleep(const struct timespec *requested, struct timespec *remaining) {
  if (!requested || requested->tv_sec < 0 || requested->tv_nsec < 0 ||
      requested->tv_nsec >= 1000000000L) { errno = EINVAL; return -1; }
  uint64_t milliseconds=(uint64_t)requested->tv_sec*1000+
    (uint64_t)(requested->tv_nsec+999999)/1000000;
  vTaskDelay(pdMS_TO_TICKS(milliseconds ? milliseconds : 1));
  if (remaining) { remaining->tv_sec=0; remaining->tv_nsec=0; }
  return 0;
}
int utimes(const char *path, const struct timeval times[2]) {
  (void)path; (void)times; errno=ENOSYS; return -1;
}
int fchmod(int fd, mode_t mode) { (void)fd; (void)mode; errno=ENOSYS; return -1; }
int fchown(int fd, uid_t owner, gid_t group) { (void)fd;(void)owner;(void)group;errno=ENOSYS;return-1; }
uid_t geteuid(void) { return 0; }
ssize_t readlink(const char *path,char *buffer,size_t size){(void)path;(void)buffer;(void)size;errno=ENOSYS;return-1;}
