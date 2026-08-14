Import("env")
import os

# platform-espressif32 6.10.0 does not propagate FS include paths while
# compiling the sibling framework SD_MMC library under deep+ LDF. Add the
# framework-owned path portably instead of embedding a workstation path.
framework_dir = env.PioPlatform().get_package_dir("framework-arduinoespressif32")
if framework_dir:
    env.Append(CPPPATH=[framework_dir + "/libraries/FS/src"])

# A firmware build must pin the backend device CA. Development receives an
# explicit non-secret test certificate file; production CI supplies its own
# generated header and signs the resulting image.
project_dir = env.subst("$PROJECT_DIR")
header = os.path.join(project_dir, "include", "PinnedDeviceCA.h")
if not os.path.exists(header):
    raise RuntimeError("PinnedDeviceCA.h is required; refusing a trust-on-first-use build")
