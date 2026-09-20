-- Repair refresh rates corrupted by the device's own Refresh-Rate header.
--
-- Until this release the header was persisted, so a device that reported one
-- of its retry intervals (the firmware's ladder starts at 15 seconds) replaced
-- the configured schedule with it, and the server handed that value straight
-- back on every poll: the device then woke every few seconds and drained its
-- battery. The admin UI never allowed anything below 60 seconds, so a smaller
-- value can only have come from that path.
UPDATE devices SET refresh_rate = 900 WHERE refresh_rate < 60;
