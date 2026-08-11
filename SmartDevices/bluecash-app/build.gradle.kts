plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.beeloy.fiscal.bluecash"
    compileSdk = 35
    defaultConfig {
        applicationId = "com.beeloy.fiscal.bluecash"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        buildConfigField("String", "FISCAL_BACKEND_URL", "\"\"")
    }
    buildTypes { release { isMinifyEnabled = false } }
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
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.5")
    testImplementation("junit:junit:4.13.2")
}
