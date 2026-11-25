-- Image format served to a device: 'bmp' (1-bit BMP, what all TRMNL firmware
-- decodes) or 'png' (1-bit PNG, for devices that only take PNG). Existing
-- devices keep BMP.
ALTER TABLE devices ADD COLUMN image_format TEXT NOT NULL DEFAULT 'bmp';
