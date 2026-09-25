plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.beeloy.fiscal.bluecash"
    compileSdk = 36
    defaultConfig {
        applicationId = "org.beeloy.bluecash.prod"
        minSdk = 24
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        buildConfigField("String", "FISCAL_BACKEND_URL", "\"\"")
    }
    buildTypes { release { isMinifyEnabled = false } }
    flavorDimensions += "environment"
    productFlavors {
        create("prod") {
            dimension = "environment"
            buildConfigField("String", "FISCAL_BACKEND_URL", "\"https://api.beeloy.org\"")
            applicationId = "org.beeloy.bluecash.prod"
            buildConfigField("String", "WEB_ORIGIN", "\"https://prod.bluecash.beeloy.org\"")
        }
        create("demo") {
            dimension = "environment"
            buildConfigField("String", "FISCAL_BACKEND_URL", "\"https://demo-api.beeloy.org\"")
            applicationId = "org.beeloy.bluecash.demo"
            buildConfigField("String", "WEB_ORIGIN", "\"https://demo.bluecash.beeloy.org\"")
        }
    }
    buildFeatures { buildConfig = true }
    testOptions { unitTests.isReturnDefaultValues = true }
    compileOptions {
        isCoreLibraryDesugaringEnabled = true
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}
kotlin { jvmToolchain(17) }

dependencies {
    implementation(project(":shared"))
    implementation(files("libs/com.android.fiscal.jar", "libs/com.android.pinpad.jar"))
    implementation("org.eclipse.paho:org.eclipse.paho.client.mqttv3:1.2.5")
    implementation("org.json:json:20240303")
    implementation("org.bouncycastle:bcprov-jdk18on:1.80")
    implementation("com.google.zxing:core:3.5.3")
    implementation("org.nanohttpd:nanohttpd:2.3.1")
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.5")
    testImplementation("junit:junit:4.13.2")
}

// Do not ship hardware placeholders as store-ready fiscal software.
val verifyStoreReadiness by tasks.registering {
    doLast { throw GradleException("STORE_BLOCKED: Vendor fiscal/pinpad SDK, certified firmware and real-device HIL are required.") }
}
tasks.configureEach {
    if (name.matches(Regex("(bundle|assemble).*(Release)"))) dependsOn(verifyStoreReadiness)
}
