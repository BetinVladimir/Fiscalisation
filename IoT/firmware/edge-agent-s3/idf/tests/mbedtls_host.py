"""Build the pinned ESP-IDF mbedTLS crypto library for actual host verification."""
import os,pathlib,subprocess,sys

def crypto_dependency(build_dir):
    source=pathlib.Path(os.environ.get("MBEDTLS_SOURCE_DIR",str(pathlib.Path.home()/".platformio/packages/framework-espidf/components/mbedtls/mbedtls")))
    include=pathlib.Path(os.environ.get("MBEDTLS_INCLUDE_DIR",str(source/"include")))
    if not (include/"mbedtls/pk.h").is_file():
        raise RuntimeError("actual mbedTLS headers unavailable; set MBEDTLS_SOURCE_DIR or MBEDTLS_INCLUDE_DIR")
    override=os.environ.get("MBEDTLS_CRYPTO_LIBRARY")
    if override:
        library=pathlib.Path(override)
        if not library.is_file():raise RuntimeError("MBEDTLS_CRYPTO_LIBRARY does not exist")
        return include,library
    target=pathlib.Path(build_dir)/"mbedtls"
    subprocess.run(["cmake","-S",str(source),"-B",str(target),"-DENABLE_PROGRAMS=OFF","-DENABLE_TESTING=OFF","-DUSE_SHARED_MBEDTLS_LIBRARY=OFF"],check=True,stdout=sys.stderr)
    subprocess.run(["cmake","--build",str(target),"--target","mbedcrypto","-j",str(min(os.cpu_count() or 2,8))],check=True,stdout=sys.stderr)
    return include,target/"library/libmbedcrypto.a"

if __name__=="__main__":
    for value in crypto_dependency(sys.argv[1]):print(value)
