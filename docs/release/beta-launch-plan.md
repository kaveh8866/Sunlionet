# SunLionet Staged Public Beta Launch Procedure

## 1. Philosophy
Avoid "Big Bang" launches. Move from high-trust, low-volume cohorts to low-trust, high-volume public distribution based on deterministic success signals.

## 2. Launch Stages

### Stage 1: Private RC (Current State)
- **Target**: Internal developers and 5-10 trusted technical testers.
- **Distribution**: Direct APK/binary sharing.
- **Goal**: Catch obvious regressions and packaging errors.
- **Success Criteria**: 100% success on "Connect" journey across 5 different device models.

### Stage 2: Limited Public Beta (The Rehearsal)
- **Target**: 50-100 users from the waitlist/community.
- **Distribution**: GitHub "Pre-release" + Private Telegram group.
- **Goal**: Validate recovery logic on diverse mobile networks and hostile network transitions.
- **Success Criteria**: No unrecoverable crashes; >90% success rate on connection probes after 24 hours of background use.

### Stage 3: Controlled Public Beta
- **Target**: General public in the target region.
- **Distribution**: GitHub Release (Full) + F-Droid (Optional) + Official Website.
- **Goal**: Scalability testing and broad hardware compatibility validation.
- **Success Criteria**: Stable support volume; no new P0/P1 issues for 7 consecutive days.

### Stage 4: Broad Public Announcement
- **Target**: Global visibility.
- **Distribution**: Press release, social media, community aggregators.
- **Goal**: Establish SunLionet as a reliable censorship-circumvention tool.

## 3. Entry/Exit Gates

| From | To | Requirements |
| :--- | :--- | :--- |
| **Stage 1** | **Stage 2** | `validation-matrix.md` P0 items all PASS; `RELEASES.json` verified. |
| **Stage 2** | **Stage 3** | All P1 issues from Stage 2 resolved; bilingual docs verified by native speakers. |
| **Stage 3** | **Stage 4** | 30 days of "Stable" operation; external security audit completed. |

## 4. Stop Conditions (Emergency Brake)
If any of the following occur, pause the rollout and revert to the previous stage:
1. **P0 Security Bug**: Any data leak or encryption failure.
2. **Crash Rate**: >5% of users reporting unrecoverable app crashes.
3. **Detection Spike**: Reports of the protocol being blocked globally within 1 hour of release.

## 5. Escalation Criteria
- **Technical**: Escalate to lead developer if a bug cannot be reproduced within 4 hours.
- **Communication**: Escalate to community manager if support volume exceeds 50 tickets/hour.
