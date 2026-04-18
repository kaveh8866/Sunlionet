plugins {
	id("com.android.application")
	id("org.jetbrains.kotlin.android")
}

fun envOrProp(name: String): String =
	System.getenv(name)?.takeIf { it.isNotBlank() }
		?: (project.findProperty(name) as String?)?.takeIf { it.isNotBlank() }
		?: ""

android {
	namespace = "com.shadownet.agent"
	compileSdk = 34

	defaultConfig {
		applicationId = "com.shadownet.agent"
		minSdk = 26
		targetSdk = 34
		versionCode = 2
		versionName = "v0.2.0-beta"
		testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
		val testerMode = envOrProp("TESTER_MODE").lowercase() == "true"
		buildConfigField("boolean", "TESTER_MODE", testerMode.toString())
		buildConfigField("String", "APP_VERSION_LABEL", "\"$versionName\"")
		buildConfigField(
			"String",
			"SING_BOX_SHA256_ARM64",
			"\"${(project.findProperty("SING_BOX_SHA256_ARM64") as String?) ?: ""}\"",
		)
		buildConfigField(
			"String",
			"SING_BOX_SHA256_ARMV7",
			"\"${(project.findProperty("SING_BOX_SHA256_ARMV7") as String?) ?: ""}\"",
		)
	}

	signingConfigs {
		create("release") {
			val keystorePath = envOrProp("RELEASE_KEYSTORE")
			if (keystorePath.isNotEmpty()) {
				storeFile = file(keystorePath)
				storePassword = envOrProp("RELEASE_KEYSTORE_PASSWORD")
				keyAlias = envOrProp("RELEASE_KEY_ALIAS")
				keyPassword = envOrProp("RELEASE_KEY_PASSWORD")
			}
		}
	}

	buildTypes {
		release {
			isMinifyEnabled = false
			signingConfig = signingConfigs.getByName("release")
			proguardFiles(
				getDefaultProguardFile("proguard-android-optimize.txt"),
				"proguard-rules.pro",
			)
		}
	}

	compileOptions {
		sourceCompatibility = JavaVersion.VERSION_17
		targetCompatibility = JavaVersion.VERSION_17
	}
	kotlinOptions {
		jvmTarget = "17"
	}
	buildFeatures {
		buildConfig = true
		viewBinding = true
	}
}

tasks.register("verifyGoMobileAar") {
	doLast {
		val aar = file("libs/shadownet.aar")
		if (!aar.exists()) {
			throw GradleException("Missing Go mobile binding: ${aar.path}. Run scripts/gomobile_bind_android.ps1 to generate it.")
		}
	}
}

tasks.register("verifySingBoxPackaging") {
	doLast {
		val arm64 = file("src/main/assets/bin/sing-box-arm64")
		val armv7 = file("src/main/assets/bin/sing-box-armv7")
		if (!arm64.exists()) {
			throw GradleException("Missing sing-box asset: ${arm64.path}")
		}
		if (!armv7.exists()) {
			throw GradleException("Missing sing-box asset: ${armv7.path}")
		}
		val shaRegex = Regex("^[0-9a-fA-F]{64}$")
		val shaArm64 = (project.findProperty("SING_BOX_SHA256_ARM64") as String?)?.trim().orEmpty()
		val shaArmv7 = (project.findProperty("SING_BOX_SHA256_ARMV7") as String?)?.trim().orEmpty()
		if (!shaRegex.matches(shaArm64)) {
			throw GradleException("SING_BOX_SHA256_ARM64 missing/invalid (expected 64 hex chars)")
		}
		if (!shaRegex.matches(shaArmv7)) {
			throw GradleException("SING_BOX_SHA256_ARMV7 missing/invalid (expected 64 hex chars)")
		}
	}
}

val isReleaseInvocation = gradle.startParameter.taskNames.any { it.contains("release", ignoreCase = true) }

if (isReleaseInvocation) {
	tasks.named("preBuild").configure {
		dependsOn("verifyGoMobileAar")
		dependsOn("verifySingBoxPackaging")
	}
}

tasks.register("verifyReleaseSigning") {
	doLast {
		val required = listOf("RELEASE_KEYSTORE", "RELEASE_KEYSTORE_PASSWORD", "RELEASE_KEY_ALIAS", "RELEASE_KEY_PASSWORD")
		val missing = required.filter { envOrProp(it).isBlank() }
		if (missing.isNotEmpty()) {
			throw GradleException("Missing release signing values: ${missing.joinToString(", ")}")
		}
		val keyFile = file(envOrProp("RELEASE_KEYSTORE"))
		if (!keyFile.exists()) {
			throw GradleException("RELEASE_KEYSTORE file does not exist: ${keyFile.path}")
		}
	}
}

tasks.matching { it.name.startsWith("assembleRelease") || it.name.startsWith("bundleRelease") }.configureEach {
	dependsOn("verifyReleaseSigning")
}

dependencies {
	implementation("androidx.core:core-ktx:1.13.1")
	implementation("androidx.appcompat:appcompat:1.7.0")
	implementation("com.google.android.material:material:1.12.0")
	implementation("androidx.camera:camera-camera2:1.3.4")
	implementation("androidx.camera:camera-lifecycle:1.3.4")
	implementation("androidx.camera:camera-view:1.3.4")
	implementation("com.google.mlkit:barcode-scanning:17.3.0")
	implementation("androidx.lifecycle:lifecycle-service:2.8.4")
	implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.4")
	implementation("androidx.security:security-crypto:1.1.0-alpha06")
	implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
	val mobileAar = file("libs/shadownet.aar")
	if (mobileAar.exists()) {
		implementation(files(mobileAar))
	}

	testImplementation("junit:junit:4.13.2")
	testImplementation("org.jetbrains.kotlin:kotlin-test:1.9.24")
	androidTestImplementation("androidx.test.ext:junit:1.2.1")
	androidTestImplementation("androidx.test.espresso:espresso-core:3.6.1")
}
