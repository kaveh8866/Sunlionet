# SunLionet Contribution Guide

## 1. Introduction
Thank you for contributing to SunLionet! We welcome contributions that improve the resilience, security, and accessibility of the project.

## 2. Code of Conduct
- Be respectful and collaborative.
- Prioritize user safety and privacy in every decision.
- Follow the principle of least privilege in code and access.

## 3. How to Contribute

### Reporting Bugs
- Use the **Bug Report** template on GitHub.
- Include logs (via "Diagnostic Export") if applicable.
- Do NOT report security vulnerabilities via GitHub issues; follow the [Vulnerability Response Lifecycle](vulnerability-response.md).

### Suggesting Enhancements
- Start a discussion in the **GitHub Discussions** or the community Telegram group.
- Provide a clear use case for the target region.

### Code Contributions
1. **Fork** the repository.
2. **Branch**: Create a feature branch (`feat/your-feature` or `fix/your-fix`).
3. **Tests**: Ensure all existing tests pass (`.\scripts\run_tests.ps1`). Add new tests for your changes.
4. **Pull Request**:
    - Describe the change and the rationale.
    - Reference any related issues.
    - Keep PRs focused; avoid large, unrelated changes.
5. **Review**: All PRs require at least one maintainer review. Security-sensitive modules require SRG review.

## 4. Security-Sensitive Contributions
Changes affecting the following areas require higher scrutiny:
- Cryptographic implementations.
- Signature and bundle verification logic.
- Update and distribution mechanisms.
- Local state management (`SecureStore`).

For these changes, include a **Threat Model** section in your PR description.

## 5. Documentation Contributions
- We highly value localization contributions, especially for languages spoken in high-risk regions.
- Ensure technical accuracy in installation and safety guides.
- Maintain the bilingual (EN/FA) baseline.

## 6. Issue Reporting Hygiene
- Check for existing issues before opening a new one.
- Use descriptive titles.
- Label your issues (e.g., `bug`, `enhancement`, `documentation`).

## 7. Contributor Onboarding
- New contributors are encouraged to start with `good-first-issue` labels.
- Frequent, high-quality contributors may be invited to become **Reviewers** or **Maintainers** as defined in the [Governance Model](model.md).
