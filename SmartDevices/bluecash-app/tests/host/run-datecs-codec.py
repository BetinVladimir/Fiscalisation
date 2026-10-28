#!/usr/bin/env python3
"""Compile actual DatecsProtocol.kt with pinned cached JVM compiler; no Android SDK/network."""
import os
from pathlib import Path
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
CACHE = Path(os.environ.get("GRADLE_USER_HOME", Path.home() / ".gradle")) / "caches/modules-2/files-2.1"
VERSION = "2.0.21"  # Match SmartDevices/build.gradle.kts.

def jar(group, artifact, version):
    matches = sorted((CACHE / group / artifact / version).glob(f"*/{artifact}-{version}.jar"))
    if len(matches) != 1:
        raise SystemExit(f"Missing/ambiguous pinned JVM dependency {group}:{artifact}:{version}; populate Gradle cache first")
    return str(matches[0])

stdlib = jar("org.jetbrains.kotlin", "kotlin-stdlib", VERSION)
annotations = jar("org.jetbrains", "annotations", "13.0")
compiler = [jar("org.jetbrains.kotlin", a, VERSION) for a in
            ("kotlin-compiler-embeddable", "kotlin-script-runtime", "kotlin-reflect")]
compiler += [stdlib, annotations, jar("org.jetbrains.intellij.deps", "trove4j", "1.0.20200330"),
             jar("org.jetbrains.kotlinx", "kotlinx-coroutines-core-jvm", "1.6.4")]
java = str(Path(os.environ["JAVA_HOME"]) / "bin/java") if "JAVA_HOME" in os.environ else "java"
src = HERE.parents[1] / "src/main/kotlin/com/beeloy/fiscal/bluecash"
# DatecsProtocol.kt references SalePayment from BlueCashEngine.kt; include both.
sources = [src / "DatecsProtocol.kt", src / "BlueCashEngine.kt", HERE / "DatecsCodecVendorHost.kt"]
with tempfile.TemporaryDirectory(prefix="beefiscal-datecs-kotlin-") as output:
    subprocess.run([java, "-cp", os.pathsep.join(compiler), "org.jetbrains.kotlin.cli.jvm.K2JVMCompiler",
                    "-no-stdlib", "-no-reflect", "-classpath", os.pathsep.join([stdlib, annotations]),
                    "-d", output] + [str(s) for s in sources], check=True)
    subprocess.run([java, "-cp", os.pathsep.join([output, stdlib]),
                    "com.beeloy.fiscal.bluecash.DatecsCodecVendorHostKt"], check=True)
