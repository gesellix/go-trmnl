-- Add font_bundle to devices to support TRMNL Framework 3.1+ bundles.
ALTER TABLE devices ADD COLUMN font_bundle TEXT NOT NULL DEFAULT 'classic';
