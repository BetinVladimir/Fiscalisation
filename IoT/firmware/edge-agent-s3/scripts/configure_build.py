Import("env")

# platform-espressif32 6.10.0 does not propagate FS include paths while
# compiling the sibling framework SD_MMC library under deep+ LDF. Add the
# framework-owned path portably instead of embedding a workstation path.
framework_dir = env.PioPlatform().get_package_dir("framework-arduinoespressif32")
if framework_dir:
    env.Append(CPPPATH=[framework_dir + "/libraries/FS/src"])
