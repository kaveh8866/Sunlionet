package com.sunlionet.agent

import android.content.Context
import org.json.JSONObject
import java.io.File
import java.lang.reflect.Method
import java.nio.charset.StandardCharsets
import java.util.concurrent.ConcurrentHashMap

object Bridge {
    private val mobileClassNames = listOf(
        "com.sunlionet.mobile.Mobile",
        "com.sunlionet.mobile.mobile.Mobile",
    )
    private val methodCache = ConcurrentHashMap<String, Method>()

    private fun mobileClass(): Class<*> {
        var last: Throwable? = null
        for (name in mobileClassNames) {
            try {
                return Class.forName(name)
            } catch (t: Throwable) {
                last = t
            }
        }
        throw last ?: ClassNotFoundException(mobileClassNames.firstOrNull() ?: "com.sunlionet.mobile.Mobile")
    }

    private fun callMobile(methodName: String, argTypes: Array<Class<*>>, args: Array<Any?>) {
        cachedMethod(methodName, argTypes).invoke(null, *args)
    }

    private fun callMobileString(methodName: String): String {
        val out = cachedMethod(methodName, emptyArray()).invoke(null)
        return out as? String ?: ""
    }

    private fun callMobileString(methodName: String, argTypes: Array<Class<*>>, args: Array<Any?>): String {
        val out = cachedMethod(methodName, argTypes).invoke(null, *args)
        return out as? String ?: ""
    }

    private fun callMobileBytes(methodName: String): ByteArray {
        val out = cachedMethod(methodName, emptyArray()).invoke(null)
        return out as? ByteArray ?: ByteArray(0)
    }

    private fun cachedMethod(methodName: String, argTypes: Array<Class<*>>): Method {
        val key = methodName + ":" + argTypes.joinToString(",") { it.name }
        methodCache[key]?.let { return it }
        val cls = mobileClass()
        val candidates = listOf(
            methodName,
            methodName.replaceFirstChar { it.lowercase() },
        ).distinct()
        var last: Throwable? = null
        for (name in candidates) {
            try {
                val method = cls.getMethod(name, *argTypes)
                methodCache.putIfAbsent(key, method)
                return method
            } catch (t: Throwable) {
                last = t
            }
        }
        throw last ?: NoSuchMethodException("${cls.name}.$methodName")
    }

    private fun isMissingMobileMethod(t: Throwable): Boolean {
        return t is NoSuchMethodException || t.cause is NoSuchMethodException
    }

    private fun stateDir(context: Context): String = File(context.filesDir, "state").absolutePath

    private fun masterKey(context: Context): String = SecureStore(context).getOrCreateMasterKeyB64Url()

    private fun ageIdentity(context: Context): String = SecureStore(context).getOrCreateAgeIdentity()

    private fun ageRecipient(context: Context): String = SecureStore(context).getOrCreateAgeRecipient()

    private fun agentConfigJSON(context: Context, usePi: Boolean = false): String {
        val secure = SecureStore(context)
        return JSONObject().apply {
            put("state_dir", File(context.filesDir, "state").absolutePath)
            put("master_key", secure.getOrCreateMasterKeyB64Url())
            put("templates_dir", File(context.filesDir, "templates").absolutePath)
            put("poll_interval_sec", 20)
            put("config_path", File(context.filesDir, "runtime/config.json").absolutePath)
            put("use_pi", usePi)
            put("pi_timeout_ms", 1200)
            put("pi_command", "pi")
            put("trusted_signer_pub_b64url", secure.getTrustedSignerKeysCSV())
            put("age_identity", secure.getOrCreateAgeIdentity())
            put("telemetry_enabled", DiagnosticsStore(context).isDiagnosticsEnabled())
            put("telemetry_transport", "onion")
        }.toString()
    }

    fun startAgent(context: Context, usePi: Boolean = false): Result<Unit> = runCatching {
        configureRuntime()
        val cfg = agentConfigJSON(context, usePi)
        runCatching {
            callMobile("StartAgentBytes", arrayOf(ByteArray::class.java), arrayOf(cfg.toByteArray(StandardCharsets.UTF_8)))
        }.getOrElse {
            if (!isMissingMobileMethod(it)) throw it
            callMobile("StartAgent", arrayOf(String::class.java), arrayOf(cfg))
        }
        Logs.i("bridge", "agent started")
    }

    fun stopAgent(): Result<Unit> = runCatching {
        callMobile("StopAgent", emptyArray(), emptyArray())
        Logs.i("bridge", "agent stopped")
    }

    fun importBundle(path: String): Result<Unit> {
        return runCatching {
            callMobile("ImportBundle", arrayOf(String::class.java), arrayOf(path))
            Logs.i("bridge", "import bundle: $path")
        }
    }

    fun importOnboardingUri(context: Context, uri: String): Result<Unit> {
        return runCatching {
            val cfg = agentConfigJSON(context, usePi = false)
            runCatching {
                callMobile(
                    "ImportOnboardingURIWithConfigBytes",
                    arrayOf(ByteArray::class.java, ByteArray::class.java),
                    arrayOf(
                        uri.toByteArray(StandardCharsets.UTF_8),
                        cfg.toByteArray(StandardCharsets.UTF_8),
                    ),
                )
            }.getOrElse {
                if (!isMissingMobileMethod(it)) throw it
                callMobile(
                    "ImportOnboardingURIWithConfig",
                    arrayOf(String::class.java, String::class.java),
                    arrayOf(uri, cfg),
                )
            }
            Logs.i("bridge", "import onboarding uri")
        }
    }

    fun getStatus(): String {
        return try {
            runCatching {
                String(callMobileBytes("GetStatusBytes"), StandardCharsets.UTF_8)
            }.getOrElse {
                if (!isMissingMobileMethod(it)) throw it
                callMobileString("GetStatus")
            }
        } catch (e: Throwable) {
            """{"running":false,"last_error":"${e.message ?: "bridge unavailable"}"}"""
        }
    }

    private fun configureRuntime() {
        runCatching {
            callMobile(
                "ConfigureAndroidRuntime",
                arrayOf(
                    Int::class.javaPrimitiveType!!,
                    Int::class.javaPrimitiveType!!,
                    Long::class.javaPrimitiveType!!,
                ),
                arrayOf(75, 2, 96L * 1024L * 1024L),
            )
        }
    }

    fun createPersona(context: Context): Result<String> = runCatching {
        callMobileString(
            "CreatePersona",
            arrayOf(String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context)),
        )
    }

    fun listPersonas(context: Context): Result<String> = runCatching {
        callMobileString(
            "ListPersonas",
            arrayOf(String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context)),
        )
    }

    fun createDeviceJoinRequest(context: Context, personaID: String): Result<String> = runCatching {
        callMobileString(
            "CreateDeviceJoinRequest",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), personaID, ageRecipient(context)),
        )
    }

    fun approveDeviceJoinRequest(context: Context, personaID: String, joinRequest: String): Result<String> = runCatching {
        callMobileString(
            "ApproveDeviceJoinRequest",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), personaID, joinRequest),
        )
    }

    fun applyDeviceJoinPackage(context: Context, personaID: String, joinPackage: String): Result<String> = runCatching {
        callMobileString(
            "ApplyDeviceJoinPackage",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), personaID, joinPackage, ageIdentity(context)),
        )
        "success"
    }

    fun deviceLinkSAS(linkBundle: String): Result<String> = runCatching {
        callMobileString("DeviceLinkSAS", arrayOf(String::class.java), arrayOf(linkBundle))
    }

    fun createContactOffer(context: Context, personaID: String, ttlSec: Int): Result<String> = runCatching {
        callMobileString(
            "CreateContactOffer",
            arrayOf(String::class.java, String::class.java, String::class.java, Int::class.javaPrimitiveType!!),
            arrayOf(stateDir(context), masterKey(context), personaID, ttlSec),
        )
    }

    fun chatAddContactFromOffer(context: Context, alias: String, offer: String): Result<String> = runCatching {
        callMobileString(
            "ChatAddContactFromOffer",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), alias, offer),
        )
    }

    fun chatList(context: Context): Result<String> = runCatching {
        callMobileString(
            "ChatList",
            arrayOf(String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context)),
        )
    }

    fun chatMessages(context: Context, chatID: String): Result<String> = runCatching {
        callMobileString(
            "ChatMessages",
            arrayOf(String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), chatID),
        )
    }

    fun chatSendText(
        relayURL: String,
        context: Context,
        personaID: String,
        contactID: String,
        text: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatSendText",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, contactID, text),
        )
    }

    fun chatSyncOnce(
        relayURL: String,
        context: Context,
        personaID: String,
        waitSec: Int = 10,
        limit: Int = 50,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatSyncOnce",
            arrayOf(
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                Int::class.javaPrimitiveType!!,
                Int::class.javaPrimitiveType!!,
            ),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, waitSec, limit),
        )
    }

    fun chatCreateGroup(context: Context, title: String, memberIDsCSV: String): Result<String> = runCatching {
        callMobileString(
            "ChatCreateGroup",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), title, memberIDsCSV),
        )
    }

    fun chatInviteToGroup(
        relayURL: String,
        context: Context,
        personaID: String,
        groupID: String,
        inviteeContactID: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatInviteToGroup",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, groupID, inviteeContactID),
        )
    }

    fun chatJoinGroup(
        relayURL: String,
        context: Context,
        personaID: String,
        groupID: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatJoinGroup",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, groupID),
        )
    }

    fun chatSetGroupRole(
        relayURL: String,
        context: Context,
        personaID: String,
        groupID: String,
        subjectContactID: String,
        role: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatSetGroupRole",
            arrayOf(
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
            ),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, groupID, subjectContactID, role),
        )
    }

    fun chatRemoveFromGroup(
        relayURL: String,
        context: Context,
        personaID: String,
        groupID: String,
        subjectContactID: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatRemoveFromGroup",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, groupID, subjectContactID),
        )
    }

    fun chatSendGroupText(
        relayURL: String,
        context: Context,
        personaID: String,
        groupID: String,
        text: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatSendGroupText",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, groupID, text),
        )
    }

    fun chatCreateCommunityRoom(
        context: Context,
        title: String,
        communityID: String,
        roomID: String,
        memberIDsCSV: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatCreateCommunityRoom",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), title, communityID, roomID, memberIDsCSV),
        )
    }

    fun chatSendCommunityPost(
        relayURL: String,
        context: Context,
        personaID: String,
        communityID: String,
        roomID: String,
        text: String,
    ): Result<String> = runCatching {
        callMobileString(
            "ChatSendCommunityPost",
            arrayOf(
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
            ),
            arrayOf(relayURL, stateDir(context), masterKey(context), personaID, communityID, roomID, text),
        )
    }

    fun communityList(context: Context): Result<String> = runCatching {
        callMobileString(
            "CommunityList",
            arrayOf(String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context)),
        )
    }

    fun communityCreate(context: Context, communityID: String): Result<String> = runCatching {
        callMobileString(
            "CommunityCreate",
            arrayOf(String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), communityID),
        )
    }

    fun communityCreateInvite(
        context: Context,
        personaID: String,
        communityID: String,
        ttlSec: Int = 86400,
        maxUses: Int = 1,
    ): Result<String> = runCatching {
        callMobileString(
            "CommunityCreateInvite",
            arrayOf(
                String::class.java,
                String::class.java,
                String::class.java,
                String::class.java,
                Int::class.javaPrimitiveType!!,
                Int::class.javaPrimitiveType!!,
            ),
            arrayOf(stateDir(context), masterKey(context), personaID, communityID, ttlSec, maxUses),
        )
    }

    fun communityCreateJoinRequest(context: Context, personaID: String, invite: String): Result<String> = runCatching {
        callMobileString(
            "CommunityCreateJoinRequest",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), personaID, invite),
        )
    }

    fun communityApproveJoin(
        context: Context,
        personaID: String,
        invite: String,
        joinRequest: String,
        role: String = "member",
    ): Result<String> = runCatching {
        callMobileString(
            "CommunityApproveJoin",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), personaID, invite, joinRequest, role),
        )
    }

    fun communityApplyJoin(
        context: Context,
        personaID: String,
        invite: String,
        joinRequest: String,
        approval: String,
    ): Result<String> = runCatching {
        callMobileString(
            "CommunityApplyJoin",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), personaID, invite, joinRequest, approval),
        )
    }

    fun aiInvoke(context: Context, requestJSON: String, localURL: String, remoteURL: String): Result<String> = runCatching {
        callMobileString(
            "AIInvoke",
            arrayOf(String::class.java, String::class.java, String::class.java, String::class.java, String::class.java),
            arrayOf(stateDir(context), masterKey(context), requestJSON, localURL, remoteURL),
        )
    }
}
