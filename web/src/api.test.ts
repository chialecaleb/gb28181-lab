import { describe, it, expect } from "vitest";
import { hexRows, status } from "./api";
describe("wire evidence display", () => {
  it("keeps byte offsets including partial final rows", () => {
    expect(hexRows("d6d0cec4")).toBe("000000  d6 d0 ce c4");
    expect(hexRows("00".repeat(17))).toContain("000010  00");
  });
  it("does not turn unknown capability states into passes", () => {
    expect(status("unknown")).toBe("未观测");
    expect(status("not-run")).toBe("未验证");
    expect(status("unrecognised")).toBe("unrecognised");
  });
});
