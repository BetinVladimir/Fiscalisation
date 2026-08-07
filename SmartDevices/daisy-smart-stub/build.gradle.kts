plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.beeloy.fiscal.daisy"
    compileSdk = 35
    defaultConfig {
        applicationId = "com.beeloy.fiscal.daisy.stub"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
        buildConfigField("boolean", "STUB_ADAPTER", "true")
    }
    buildFeatures { buildConfig = true }
    buildTypes {
        debug { applicationIdSuffix = ".debug" }
        release {
            // The application still compiles for contract verification, but
            // runtime construction hard-fails for environment=prod.
            isMinifyEnabled = false
        }
    }
    testOptions { unitTests.isReturnDefaultValues = true }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}
kotlin { jvmToolchain(17) }

dependencies {
    implementation(project(":shared"))
    testImplementation("junit:junit:4.13.2")
}
