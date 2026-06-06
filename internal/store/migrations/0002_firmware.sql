-- Per-device firmware-update and special-function controls, surfaced in the
-- /api/display response. The firmware/reset flags are one-shot: the server
-- clears them after serving once to avoid boot loops.

ALTER TABLE devices ADD COLUMN firmware_update  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE devices ADD COLUMN firmware_url     TEXT;
ALTER TABLE devices ADD COLUMN reset_firmware   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE devices ADD COLUMN special_function TEXT NOT NULL DEFAULT 'none';
