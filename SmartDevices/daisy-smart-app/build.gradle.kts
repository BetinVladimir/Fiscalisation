plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.beeloy.fiscal.daisy"
    compileSdk = 36
    defaultConfig {
        applicationId = "org.beeloy.daisy.prod"
        minSdk = 24
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
        buildConfigField("boolean", "STUB_ADAPTER", "true")
    }
    flavorDimensions += "environment"
    productFlavors {
        create("prod") {
            dimension = "environment"
            applicationId = "org.beeloy.daisy.prod"
            buildConfigField("String", "WEB_ORIGIN", "\"https://prod.daisy.beeloy.org\"")
        }
        create("demo") {
            dimension = "environment"
            applicationId = "org.beeloy.daisy.demo"
            buildConfigField("String", "WEB_ORIGIN", "\"https://demo.daisy.beeloy.org\"")
        }
    }
    buildFeatures { buildConfig = true }
    buildTypes {
        debug { applicationIdSuffix = ".debug" }
        release { isMinifyEnabled = false }
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

// Do not ship hardware placeholders as store-ready fiscal software.
val verifyStoreReadiness by tasks.registering {
    doLast { throw GradleException("STORE_BLOCKED: Daisy is a debug-only vendor stub without a launcher; vendor SDK and a real application are required.") }
}
tasks.configureEach {
    if (name.matches(Regex("(bundle|assemble).*(Release)"))) dependsOn(verifyStoreReadiness)
}
