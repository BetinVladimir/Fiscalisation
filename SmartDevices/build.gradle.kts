plugins {
    id("com.android.application") version "8.8.2" apply false
    id("com.android.library") version "8.8.2" apply false
    id("org.jetbrains.kotlin.android") version "2.0.21" apply false
}

// Keep Kotlin formatting reproducible and independent from a developer IDE.
val ktfmt by configurations.creating

dependencies {
    ktfmt("com.facebook:ktfmt:0.40")
}

tasks.register<JavaExec>("formatBlueCashKotlin") {
    group = "formatting"
    description = "Formats all BlueCash application and test Kotlin sources."
    classpath = ktfmt
    mainClass.set("com.facebook.ktfmt.cli.Main")
    args("--google-style")
    args(
        fileTree("bluecash-app") {
            include("src/**/*.kt")
            exclude("build/**")
        }.files.sorted().map { it.absolutePath },
    )
}
