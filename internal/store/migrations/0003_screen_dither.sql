-- Per-screen dithering override. NULL means "inherit the global dither_mode
-- setting"; otherwise holds a token render.ParseMode understands
-- ("threshold" or "floyd_steinberg").
ALTER TABLE screens ADD COLUMN dither_mode TEXT;
