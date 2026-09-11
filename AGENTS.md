# Agent Guidelines

## Tests

Do not write policy assertion tests.

It is acceptable to set configuration to a particular value and test whether the resulting behavior works or fails.

It is not acceptable to test how configuration itself is set, including shipped defaults, environment-specific values, or whether a feature is enabled or disabled by default.

Test behavior under configuration, not configuration policy.
