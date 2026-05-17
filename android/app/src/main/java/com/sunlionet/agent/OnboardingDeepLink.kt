package com.sunlionet.agent

object OnboardingDeepLink {
    const val URI_PREFIX = "sunlionet://config/"
    private const val SHORT_PREFIX = "SL1:"
    private const val MAX_PAYLOAD_CHARS = 300
    private const val MAX_URI_CHARS = URI_PREFIX.length + MAX_PAYLOAD_CHARS
    private val tokenPattern = Regex("^[A-Za-z0-9_-]{1,$MAX_PAYLOAD_CHARS}$")

    fun normalize(raw: String): String? {
        val compact = raw.trim()
            .replace("\r", "")
            .replace("\n", "")
            .replace(" ", "")
        if (compact.isBlank() || compact.length > MAX_URI_CHARS) {
            return null
        }
        val token = when {
            compact.startsWith(URI_PREFIX) -> compact.removePrefix(URI_PREFIX)
            compact.startsWith(SHORT_PREFIX) -> compact.removePrefix(SHORT_PREFIX)
            else -> return null
        }
        if (!tokenPattern.matches(token)) {
            return null
        }
        return URI_PREFIX + token
    }

    fun isOnboarding(raw: String): Boolean = normalize(raw) != null
}
