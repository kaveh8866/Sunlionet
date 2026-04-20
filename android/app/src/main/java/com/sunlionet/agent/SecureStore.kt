package com.sunlionet.agent

import android.content.Context
import android.util.Base64
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.security.KeyPairGenerator
import java.security.SecureRandom

class SecureStore(context: Context) {
    private val prefs = runCatching {
        val masterKey = MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        EncryptedSharedPreferences.create(
            context,
            "SUNLIONET_secure",
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }.getOrElse { err ->
        throw IllegalStateException("secure storage initialization failed", err)
    }

    fun ensureDefaultTrustAnchors() {
        if (getTrustedSignerKeysCSV().isBlank()) {
            setTrustedSignerKeysCSV(DEFAULT_TRUSTED_SIGNER_PUB_B64URL)
        }
        getOrCreateAgeIdentity()
        getOrCreateMasterKeyB64Url()
    }

    fun getOrCreateMasterKeyB64Url(): String {
        val existing = prefs.getString("master_key_b64url", null)
        if (!existing.isNullOrBlank()) {
            return existing
        }
        val key = ByteArray(32)
        SecureRandom().nextBytes(key)
        val b64 = b64UrlNoPad(key)
        prefs.edit().putString("master_key_b64url", b64).apply()
        return b64
    }

    fun getOrCreateAgeIdentity(): String {
        val existing = prefs.getString("age_identity", null)
        if (!existing.isNullOrBlank()) {
            return existing
        }
        val id = generateAgeIdentity()
        prefs.edit().putString("age_identity", id).apply()
        return id
    }

    fun getOrCreateAgeRecipient(): String {
        getOrCreateAgeIdentity()
        val rec = prefs.getString("age_recipient", null)
        if (!rec.isNullOrBlank()) {
            return rec
        }
        val id = generateAgeIdentity()
        prefs.edit().putString("age_identity", id).apply()
        return prefs.getString("age_recipient", "") ?: ""
    }

    fun getTrustedSignerKeysCSV(): String {
        return prefs.getString("trusted_signers_b64url_csv", "") ?: ""
    }

    fun setTrustedSignerKeysCSV(value: String) {
        prefs.edit().putString("trusted_signers_b64url_csv", value).apply()
    }

    fun setDesiredConnected(desired: Boolean) {
        prefs.edit().putBoolean("desired_connected", desired).apply()
    }

    fun isDesiredConnected(): Boolean {
        return prefs.getBoolean("desired_connected", false)
    }

    fun isAdvancedModeEnabled(): Boolean {
        return prefs.getBoolean("advanced_mode", false)
    }

    fun setAdvancedModeEnabled(enabled: Boolean) {
        prefs.edit().putBoolean("advanced_mode", enabled).apply()
    }

    fun setLastError(msg: String) {
        prefs.edit().putString("last_error", msg).apply()
    }

    fun getLastError(): String {
        return prefs.getString("last_error", "") ?: ""
    }

    private fun b64UrlNoPad(b: ByteArray): String {
        return Base64.encodeToString(b, Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)
    }

    private fun generateAgeIdentity(): String {
        val kpg = KeyPairGenerator.getInstance("X25519")
        val kp = kpg.generateKeyPair()
        val privEncoded = kp.private.encoded
        val pubEncoded = kp.public.encoded
        if (privEncoded.size < 32 || pubEncoded.size < 32) {
            throw IllegalStateException("X25519 key encoding too short")
        }
        val priv = privEncoded.takeLast(32).toByteArray()
        val pub = pubEncoded.takeLast(32).toByteArray()
        val secret = Bech32.encode("age-secret-key", Bech32.convertBits(priv, 8, 5, true))
        val recipient = Bech32.encode("age", Bech32.convertBits(pub, 8, 5, true))
        prefs.edit().putString("age_recipient", recipient).apply()
        return secret.uppercase()
    }

    companion object {
        private const val DEFAULT_TRUSTED_SIGNER_PUB_B64URL = "A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg"
    }

    private object Bech32 {
        private const val CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
        private val GEN = intArrayOf(0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3)

        fun encode(hrp: String, data: ByteArray): String {
            val checksum = createChecksum(hrp, data)
            val combined = ByteArray(data.size + checksum.size)
            System.arraycopy(data, 0, combined, 0, data.size)
            System.arraycopy(checksum, 0, combined, data.size, checksum.size)
            val sb = StringBuilder(hrp.length + 1 + combined.size)
            sb.append(hrp)
            sb.append('1')
            for (b in combined) {
                sb.append(CHARSET[b.toInt()])
            }
            return sb.toString()
        }

        fun convertBits(data: ByteArray, fromBits: Int, toBits: Int, pad: Boolean): ByteArray {
            var acc = 0
            var bits = 0
            val maxv = (1 shl toBits) - 1
            val out = ArrayList<Byte>()
            for (value in data) {
                val v = value.toInt() and 0xff
                if (v ushr fromBits != 0) {
                    throw IllegalArgumentException("invalid data range: $v")
                }
                acc = (acc shl fromBits) or v
                bits += fromBits
                while (bits >= toBits) {
                    bits -= toBits
                    out.add(((acc ushr bits) and maxv).toByte())
                }
            }
            if (pad) {
                if (bits > 0) {
                    out.add(((acc shl (toBits - bits)) and maxv).toByte())
                }
            } else {
                if (bits >= fromBits) {
                    throw IllegalArgumentException("illegal zero padding")
                }
                if (((acc shl (toBits - bits)) and maxv) != 0) {
                    throw IllegalArgumentException("non-zero padding")
                }
            }
            return out.toByteArray()
        }

        private fun createChecksum(hrp: String, data: ByteArray): ByteArray {
            val values = hrpExpand(hrp) + data + ByteArray(6)
            var polymod = polymod(values) xor 1
            val ret = ByteArray(6)
            for (i in 0 until 6) {
                ret[i] = ((polymod ushr (5 * (5 - i))) and 31).toByte()
            }
            return ret
        }

        private fun hrpExpand(hrp: String): ByteArray {
            val out = ByteArray(hrp.length * 2 + 1)
            var i = 0
            for (c in hrp) {
                out[i++] = (c.code ushr 5).toByte()
            }
            out[i++] = 0
            for (c in hrp) {
                out[i++] = (c.code and 31).toByte()
            }
            return out
        }

        private fun polymod(values: ByteArray): Int {
            var chk = 1
            for (b in values) {
                val top = chk ushr 25
                chk = (chk and 0x1ffffff) shl 5 xor (b.toInt() and 0xff)
                for (i in 0 until 5) {
                    if (((top ushr i) and 1) != 0) {
                        chk = chk xor GEN[i]
                    }
                }
            }
            return chk
        }
    }
}
