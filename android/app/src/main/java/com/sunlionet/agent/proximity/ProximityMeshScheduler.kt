package com.sunlionet.agent.proximity

import android.app.job.JobInfo
import android.app.job.JobParameters
import android.app.job.JobScheduler
import android.app.job.JobService
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.PowerManager
import com.sunlionet.agent.AgentService
import com.sunlionet.agent.Logs

class ProximityMeshScheduler(
    private val context: Context,
) {
    fun schedule() {
        val scheduler = context.getSystemService(JobScheduler::class.java) ?: return
        val job = JobInfo.Builder(JOB_ID, ComponentName(context, ProximityMeshJobService::class.java))
            .setPersisted(true)
            .setRequiredNetworkType(JobInfo.NETWORK_TYPE_NONE)
            .setPeriodic(15 * 60_000L)
            .build()
        scheduler.schedule(job)
    }

    fun cancel() {
        context.getSystemService(JobScheduler::class.java)?.cancel(JOB_ID)
    }

    fun runWindow(controller: ProximityController, windowMs: Long = 25_000L) {
        val pm = context.getSystemService(PowerManager::class.java)
        val wl = pm?.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "SunLionet:BleMesh")
        runCatching { wl?.acquire(windowMs + 2_000L) }
        controller.start()
        android.os.Handler(context.mainLooper).postDelayed({
            runCatching { controller.stop() }
            if (wl?.isHeld == true) {
                runCatching { wl.release() }
            }
        }, windowMs)
    }

    companion object {
        const val JOB_ID = 23041
    }
}

class ProximityMeshJobService : JobService() {
    override fun onStartJob(params: JobParameters?): Boolean {
        Logs.i("proximity", "scheduled mesh window")
        startService(Intent(this, AgentService::class.java).apply {
            action = AgentService.ACTION_PROXIMITY_WINDOW
        })
        jobFinished(params, false)
        return false
    }

    override fun onStopJob(params: JobParameters?): Boolean = true
}
