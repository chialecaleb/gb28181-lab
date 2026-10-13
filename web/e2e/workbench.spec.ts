import { test, expect } from "@playwright/test";
import { readFile, mkdir } from "node:fs/promises";
test("real SIP workbench, keyboard, configuration and evidence", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "连接本地实验工作台" }),
  ).toBeVisible();
  const token = (
    await readFile("../.local/ui-test/access.token", "utf8")
  ).trim();
  await page.getByLabel("本机访问令牌").fill(token);
  await page.getByLabel("本机访问令牌").press("Enter");
  await expect(page.getByText("引擎已连接", { exact: false })).toBeVisible();
  await expect(page.locator(".context")).toContainText("标准模式");
  await page.getByRole("button", { name: "配置设备与绑定" }).click();
  await page
    .getByLabel("协议模式", { exact: true })
    .selectOption("compatibility");
  await page.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(page.locator(".context")).toContainText("兼容模式");
  await expect(page.locator(".warning")).toContainText(
    "内置平台没有需要启用的兼容项",
  );
  await mkdir("../artifacts/interoperability-validation", { recursive: true });
  await page.screenshot({
    path: "../artifacts/interoperability-validation/workbench-compatibility.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "配置设备与绑定" }).click();
  await expect(page.getByLabel("协议模式", { exact: true })).toHaveValue(
    "compatibility",
  );
  await page.getByLabel("协议模式", { exact: true }).selectOption("standard");
  await page.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(page.locator(".context")).toContainText("标准模式");
  await expect(
    page.getByRole("button", { name: "Catalog 查询", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "注册 / 刷新", exact: true }).click();
  await expect(page.locator(".parameters")).toContainText(
    "注册 / 刷新 · 已完成",
  );
  for (const name of ["Keepalive 心跳", "DeviceInfo 查询", "Catalog 查询"]) {
    await page.getByRole("button", { name, exact: true }).click();
    await expect(page.locator(".parameters")).toContainText(name + " · 已完成");
  }
  await page.getByRole("tab", { name: "分层验证", exact: true }).click();
  await expect(page.locator(".checks")).toContainText("通过");
  await expect(page.locator(".checks")).toContainText("未验证");
  await page.getByRole("tab", { name: "报文原文", exact: true }).click();
  await page.getByLabel("按操作筛选").selectOption("");
  await expect(page.locator(".tracelist")).toContainText("REGISTER");
  await page.getByRole("tab", { name: "十六进制", exact: true }).click();
  await expect(page.locator(".tracedetail pre")).toContainText("000000");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await mkdir("../artifacts/m0/ui", { recursive: true });
  await page.screenshot({
    path: "../artifacts/m0/ui/workbench-1280.png",
    fullPage: true,
  });
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出脱敏证据" }).click();
  const file = await download;
  await file.saveAs("../artifacts/m0/ui/evidence.json");
  const exported = await readFile("../artifacts/m0/ui/evidence.json", "utf8");
  expect(exported.includes(token)).toBe(false);
  expect(exported).toContain("sha256");
  expect(exported).toContain('"protocolMode": "standard"');
  await page.getByRole("button", { name: "配置设备与绑定" }).click();
  await expect(
    page.getByRole("button", { name: "保存配置", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "关闭", exact: true }).press("Enter");
  await page.getByRole("button", { name: "注销", exact: true }).click();
  await expect(page.locator(".parameters")).toContainText("注销 · 已完成");
  await page.getByRole("button", { name: "配置设备与绑定" }).click();
  await page.getByLabel("设备名称", { exact: true }).fill("中文实验设备𠀀");
  await page.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator(".tree")).toContainText("中文实验设备𠀀");
  await page.getByRole("button", { name: "注册 / 刷新", exact: true }).click();
  await expect(page.locator(".parameters")).toContainText(
    "注册 / 刷新 · 已完成",
  );
  await page
    .getByRole("button", { name: "DeviceInfo 查询", exact: true })
    .click();
  await expect(page.locator(".parameters")).toContainText(
    "DeviceInfo 查询 · 已完成",
  );
  await page.getByRole("tab", { name: "状态对比", exact: true }).click();
  await expect(page.locator(".tracedetail")).toContainText("中文实验设备𠀀");
  await page.getByRole("button", { name: "注销", exact: true }).click();
  await expect(page.locator(".parameters")).toContainText("注销 · 已完成");
  const demo = page.getByRole("button", { name: "完整信令演示", exact: true });
  await demo.click();
  await expect(demo).toBeDisabled();
  await expect(demo).toBeEnabled({ timeout: 15000 });
  await expect(page.locator(".parameters")).toContainText("注销 · 已完成");
  await page.getByRole("button", { name: "配置设备与绑定" }).click();
  await page.getByRole("button", { name: "关闭", exact: true }).press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(errors).toEqual([]);
});
