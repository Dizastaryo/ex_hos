-- Track OTP-verify attempts per code so we can lock out brute-force after
-- N failed tries. Defaults to 0 for existing rows (mostly already expired).
ALTER TABLE otp_codes ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;
