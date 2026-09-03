// SPDX-License-Identifier: Apache-2.0

// Available Paladin identities offered as party choices in the FX agreement form.
// `configured` is false when the backend roster is empty/unavailable, so the UI
// can distinguish "roster not configured" from a genuinely empty result rather
// than presenting a silent empty dropdown.
export interface ListIdentitiesResponse {
  identities: string[];
  configured?: boolean;
}

// Parsed identity roster returned to the store: the identities plus whether the
// backend reported the roster as configured.
export interface IdentityRoster {
  identities: string[];
  configured: boolean;
}
