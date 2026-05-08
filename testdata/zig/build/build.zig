const std = @import("std");

pub fn build(b: *std.Build) void {
    const exe = b.addExecutable(.{ .name = "demo" });
    exe.addCSourceFile("src/main.c", &[_][]const u8{});
    b.installArtifact(exe);
}
