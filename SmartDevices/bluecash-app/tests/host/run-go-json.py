#!/usr/bin/env python3
"""Actual Go encoding/json/domain structs versus production Kotlin, using pinned cached compiler."""
import os
import shutil
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
source = HERE.parents[1] / "src/main/kotlin/com/beeloy/fiscal/bluecash"
jsonjar = jar("org.json", "json", "20240303")
repo = Path(os.environ.get("FISCAL_REPO_ROOT", str(HERE.parents[3])))
sources = [str(source / "CanonicalJson.kt"), str(source / "BlueCashSyncWire.kt")]
with tempfile.TemporaryDirectory(prefix="beefiscal-go-json-kotlin-") as output:
    out = Path(output)
    backend = out / "backend"
    backend.mkdir()
    for name in ["go.mod", "go.sum"]: shutil.copy2(repo / "fiscal-backend" / name, backend / name)
    for name in ["domain", "config", "provisioningseal"]:
        shutil.copytree(repo / "fiscal-backend/internal" / name, backend / "internal" / name)
    oracle = backend / "cmd/oracle"
    oracle.mkdir(parents=True)
    shutil.copy2(HERE / "go-json-oracle.go", oracle / "main.go")
    env = dict(os.environ)
    env.setdefault("GOCACHE", str(out / "go-cache"))
    with (out / "corpus.jsonl").open("w") as corpus:
        subprocess.run(["go", "run", "-mod=readonly", "./cmd/oracle"], cwd=backend, env=env, stdout=corpus, check=True)
    host = HERE / "GoJsonHost.kt"
    subprocess.run([java, "-cp", os.pathsep.join(compiler), "org.jetbrains.kotlin.cli.jvm.K2JVMCompiler",
                    "-no-stdlib", "-no-reflect", "-classpath", os.pathsep.join([stdlib, annotations, jsonjar]),
                    "-d", output] + sources + [str(host)], check=True)
    subprocess.run([java, "-cp", os.pathsep.join([output, stdlib, jsonjar]),
                    "com.beeloy.fiscal.bluecash.GoJsonHostKt", str(out / "corpus.jsonl")], check=True)
