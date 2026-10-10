import { test, expect } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { start, stop, restart, baseUrl } from "./evcc";
import type { ThermalFunction } from "../assets/js/types/evcc";
import { expectModalHidden, expectModalVisible } from "./utils";

const CONFIG = "nibe-thermal.evcc.yaml";
const simulator = "http://127.0.0.1:8080";

const write = (address: number, value: number) => {
  execFileSync(process.env["NIBE_SIM_PYTHON"] || "python3", [
    "-c",
    `
from pymodbus.client import ModbusTcpClient
with ModbusTcpClient('127.0.0.1', port=5020) as client:
    result = client.write_register(${address}, ${value}, device_id=1)
    assert not result.isError(), result
`,
  ]);
};

test.use({ baseURL: baseUrl(), hasTouch: true });
test.describe("NIBE comfort functions", () => {
  test.skip(!process.env["NIBE_SIM"], "Requires the read-only nibe-sim main branch on loopback");

  test.afterEach(async () => {
    await stop();
  });

  test("independent targets and safe restoration", async ({ page, request }) => {
    test.setTimeout(180000);
    await request.post(`${simulator}/reset`);
    write(206, 210);
    let instance = await start(CONFIG);
    const state = async () => await (await request.get("/api/state")).json();
    const effective = async () =>
      (await (await request.get(`${simulator}/state`)).json()).effective;
    const functions = async (): Promise<ThermalFunction[]> => (await state()).loadpoints[0].thermal;
    const boost = async (kind: string, enabled: boolean) => {
      const res = await request.post(`/api/loadpoints/1/thermal/${kind}/boost/${enabled}`);
      expect(res.ok(), await res.text()).toBe(true);
    };

    await request.post("/api/loadpoints/1/phases/1");
    await page.goto("/");
    const lp = page.getByTestId("loadpoint");
    await expect(lp).toHaveCount(1);
    const summary = lp.getByRole("button", { name: "Comfort settings", exact: true });
    const modal = page.getByTestId("loadpoint-settings-modal");
    const openSettings = async () => {
      await summary.click();
      await expectModalVisible(modal);
    };
    const closeSettings = async () => {
      await Promise.all([
        modal.evaluate(
          (element) =>
            new Promise<void>((resolve) => {
              element.addEventListener("hidden.bs.modal", () => resolve(), { once: true });
            })
        ),
        modal.getByRole("button", { name: "Close" }).click(),
      ]);
      await expectModalHidden(modal);
    };
    await expect(summary).toContainText("21.0°C");
    await expect(summary).toContainText("Medium");
    await expect(lp.getByRole("switch")).toHaveCount(0);
    await expect(lp.getByLabel("Boost delta")).toHaveCount(0);
    await expect(lp.getByLabel("Boost mode")).toHaveCount(0);
    const screenshots = process.env["NIBE_SCREENSHOTS"];
    if (screenshots) {
      mkdirSync(screenshots, { recursive: true });
      await lp.screenshot({ path: `${screenshots}/nibe-normal.png`, animations: "disabled" });
    }
    await openSettings();
    const room = modal.getByRole("region", { name: "Space heating", exact: true });
    const water = modal.getByRole("region", { name: "Hot water", exact: true });
    await expect(room).toContainText("21.0°C");
    await expect(water).toContainText("Medium");
    await expect(room.getByRole("slider", { name: "Normal target", exact: true })).toHaveValue(
      "21"
    );
    await expect(room.getByRole("slider", { name: "Boost target", exact: true })).toHaveValue("22");
    await expect(water.getByLabel("Boost mode")).toHaveValue("large");

    if (screenshots) {
      await modal.screenshot({ path: `${screenshots}/nibe-settings.png`, animations: "disabled" });
      const heading = await modal
        .getByRole("heading", { name: "Comfort", exact: true })
        .boundingBox();
      const lastFunction = await water.boundingBox();
      const temperatures = await room
        .getByRole("group", { name: "Temperatures", exact: true })
        .boundingBox();
      if (!heading || !lastFunction || !temperatures)
        throw new Error("Comfort controls unavailable for screenshot");
      await page.screenshot({
        path: `${screenshots}/nibe-controls.png`,
        animations: "disabled",
        clip: {
          x: heading.x - 4,
          y: heading.y - 4,
          width: Math.max(heading.width, temperatures.x + temperatures.width - heading.x) + 16,
          height: lastFunction.y + lastFunction.height - heading.y + 8,
        },
      });
    }

    await room.getByRole("switch", { name: "Comfort boost" }).check();
    await expect(room).toContainText("22.0°C");
    expect((await effective()).dhw.normal_mode).toBe("medium");
    await water.getByRole("switch", { name: "Comfort boost" }).check();
    await expect(water).toContainText("Large");
    await expect(room.getByText("Boost active", { exact: true })).toBeVisible();
    await expect(water.getByText("Boost active", { exact: true })).toBeVisible();
    expect((await functions()).every((fn) => fn.active)).toBe(true);
    expect((await state()).loadpoints).toHaveLength(1);
    expect((await functions()).some((fn) => "power" in fn || "energy" in fn)).toBe(false);
    await closeSettings();
    await expect(summary).toContainText("22.0°C");
    await expect(summary).toContainText("Large");
    await expect(summary).toContainText("Boost active");
    if (screenshots) {
      await lp.screenshot({ path: `${screenshots}/nibe-both-boosts.png`, animations: "disabled" });
      await page.setViewportSize({ width: 390, height: 1000 });
      await expect(page.getByTestId("tab-more")).toBeInViewport();
      await page.screenshot({
        path: `${screenshots}/nibe-mobile.png`,
        fullPage: true,
        animations: "disabled",
      });
      await page.setViewportSize({ width: 1400, height: 1100 });
    }
    await openSettings();
    if (screenshots) {
      await modal.screenshot({
        path: `${screenshots}/nibe-settings-active.png`,
        animations: "disabled",
      });
    }

    await room.getByRole("switch", { name: "Comfort boost" }).uncheck();
    await expect(room).toContainText("21.0°C");
    expect((await effective()).dhw.normal_mode).toBe("large");
    await water.getByRole("switch", { name: "Comfort boost" }).uncheck();
    await expect(water).toContainText("Medium");

    const boostedTarget = room.getByRole("slider", { name: "Boost target", exact: true });
    for (const target of ["22.1", "22.2", "22.3", "22.4", "22.5"]) {
      await expect(boostedTarget).toBeEnabled();
      await boostedTarget.press("ArrowRight");
      await expect(boostedTarget).toHaveValue(target);
      await expect.poll(async () => (await functions())[0]?.boostTarget).toBe(Number(target));
    }
    await boost("spaceHeating", true);
    await expect(room).toContainText("22.5°C");
    write(206, 200);
    await expect(room).toContainText("Target changed externally");
    await closeSettings();
    await expect(summary).toContainText("Target changed");
    await expect(summary).not.toContainText("Boost active");
    await openSettings();
    await boost("spaceHeating", true);
    expect((await effective()).room.normal_setpoint_c).toBe(20);
    await boost("spaceHeating", false);
    expect((await effective()).room.normal_setpoint_c).toBe(20);

    await boost("domesticHotWater", true);
    write(56, 4);
    await expect(water).toContainText("Target changed externally");
    await boost("domesticHotWater", false);
    expect((await effective()).dhw.normal_mode).toBe("smart_control");

    // Failed API inputs cannot disable normal heating or write unknown modes.
    for (const [kind, config] of [
      ["spaceHeating", { delta: -1 }],
      ["domesticHotWater", { mode: "notInUse" }],
    ]) {
      expect(
        (await request.post(`/api/loadpoints/1/thermal/${kind}/config`, { data: config })).status()
      ).toBe(400);
    }

    write(206, 210);
    write(56, 1);
    await boost("spaceHeating", true);
    await boost("domesticHotWater", true);
    instance = await restart(CONFIG);
    await page.reload();
    await openSettings();
    await expect(room).toContainText("21.0°C");
    await expect(water).toContainText("Medium");
    await expect(room.getByRole("slider", { name: "Normal target", exact: true })).toHaveValue(
      "21"
    );
    await expect(boostedTarget).toHaveValue("22.5");
    expect((await functions()).every((fn) => !fn.active && !fn.requested)).toBe(true);

    // Legacy SG Ready/power control still uses only its original registers.
    await request.post("/api/loadpoints/1/phases/1");
    await request.post("/api/loadpoints/1/maxcurrent/20");
    await request.post("/api/loadpoints/1/mode/now");
    await expect
      .poll(async () => (await state()).loadpoints[0].chargePower, { timeout: 45000 })
      .toBe(4500);
    await expect.poll(async () => (await state()).homePower).toBe(500);
    await boost("spaceHeating", true);
    await boost("domesticHotWater", true);
    await expect.poll(async () => (await state()).homePower).toBe(500);
    expect((await state()).loadpoints[0].chargePower).toBe(4500);
    await request.post("/api/loadpoints/1/maxcurrent/6");
    await expect
      .poll(async () => (await state()).loadpoints[0].chargePower, { timeout: 45000 })
      .toBe(1300);
    await request.post("/api/loadpoints/1/mode/off");
    await expect
      .poll(async () => (await state()).loadpoints[0].chargePower, { timeout: 45000 })
      .toBe(1500);
    expect((await effective()).room.normal_setpoint_c).toBe(22.5);
    expect((await effective()).dhw.normal_mode).toBe("large");

    // A hard process stop exercises the durable ownership journal.
    if (!instance) throw new Error("evcc instance unavailable");
    instance.kill("SIGKILL");
    await restart(CONFIG, "--disable-auth", true);
    await expect.poll(async () => (await effective()).room.normal_setpoint_c).toBe(21);
    await expect.poll(async () => (await effective()).dhw.normal_mode).toBe("medium");
    const events = (await (await request.get(`${simulator}/events`)).json()).events;
    expect(events.some((event: { address: number }) => event.address === 2740)).toBe(false);
    expect(events.some((event: { address: number }) => event.address === 6008)).toBe(true);
    expect(events.some((event: { address: number }) => event.address === 6007)).toBe(true);
    expect(events.some((event: { address: number }) => event.address === 2741)).toBe(true);

    await page.setViewportSize({ width: 390, height: 1000 });
    await page.goto("/?lang=de");
    const germanSummary = lp.getByRole("button", { name: "Komforteinstellungen", exact: true });
    await expect(germanSummary).toContainText("Raumheizung");
    await expect(germanSummary).toContainText("Warmwasser");
    await expect(germanSummary).toContainText("21,0 °C");
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)
    ).toBe(true);
    if (screenshots) {
      await expect(page.getByTestId("tab-more")).toBeInViewport();
      await page.screenshot({
        path: `${screenshots}/nibe-mobile-de.png`,
        fullPage: true,
        animations: "disabled",
      });
    }
    await germanSummary.click();
    await expectModalVisible(modal);
    await expect(modal.getByRole("region", { name: "Raumheizung", exact: true })).toBeVisible();
    await expect(modal.getByRole("region", { name: "Warmwasser", exact: true })).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)
    ).toBe(true);
    if (screenshots) {
      await page.screenshot({
        path: `${screenshots}/nibe-settings-mobile-de.png`,
        fullPage: true,
        animations: "disabled",
      });
    }
  });

  test("editable normal and boost temperatures", async ({ page, request }) => {
    await request.post(`${simulator}/reset`);
    write(206, 210);
    await start(CONFIG);
    await page.goto("/");
    const lp = page.getByTestId("loadpoint");
    const modal = page.getByTestId("loadpoint-settings-modal");
    const summary = lp.getByRole("button", { name: "Comfort settings", exact: true });
    const open = async () => {
      await summary.click();
      await expectModalVisible(modal);
    };
    const current = async () =>
      (await (await request.get(`${simulator}/state`)).json()).effective.room.normal_setpoint_c;
    const targets = async (): Promise<ThermalFunction> =>
      (await (await request.get("/api/loadpoints/1/thermal")).json())[0];
    await open();
    const room = modal.getByRole("region", { name: "Space heating", exact: true });
    const normal = room.getByRole("slider", { name: "Normal target", exact: true });
    const boosted = room.getByRole("slider", { name: "Boost target", exact: true });
    const toggle = room.getByRole("switch", { name: "Comfort boost" });
    await normal.press("ArrowLeft");
    await expect(normal).toHaveValue("20.9");
    await expect.poll(current).toBe(20.9);
    await expect(boosted).toBeEnabled();
    await boosted.press("ArrowRight");
    await expect(boosted).toHaveValue("22.1");
    await expect.poll(async () => (await targets()).boostTarget).toBe(22.1);
    await expect(toggle).toBeEnabled();
    expect(await current()).toBe(20.9);
    await toggle.check();
    await expect.poll(current).toBe(22.1);
    await expect(normal).toBeEnabled();
    await expect(boosted).toBeEnabled();
    await normal.press("ArrowLeft");
    await expect(normal).toHaveValue("20.8");
    await expect.poll(async () => (await targets()).normalTarget).toBe(20.8);
    await expect(boosted).toBeEnabled();
    expect(await current()).toBe(22.1);
    await boosted.press("ArrowRight");
    await expect(boosted).toHaveValue("22.2");
    await expect.poll(current).toBe(22.2);
    await expect(toggle).toBeEnabled();
    await toggle.uncheck();
    await expect.poll(current).toBe(20.8);
    await restart(CONFIG);
    await page.reload();
    await open();
    await expect(normal).toHaveValue("20.8");
    await expect(boosted).toHaveValue("22.2");

    const bounds = await boosted.boundingBox();
    if (!bounds) throw new Error("Boost handle unavailable");
    const position = (value: number) => bounds.x + 10 + ((value - 5) / 25) * (bounds.width - 20);
    await page.mouse.move(position(22.2), bounds.y + bounds.height / 2);
    await page.mouse.down();
    await page.mouse.move(position(23), bounds.y + bounds.height / 2, { steps: 8 });
    await page.mouse.up();
    await expect(boosted).toHaveValue("23");
    expect(await current()).toBe(20.8);
    await toggle.check();
    await expect.poll(current).toBe(23);
    write(206, 205);
    const response = await request.post("/api/loadpoints/1/thermal/spaceHeating/targets", {
      data: { normal: 21, boost: 23 },
    });
    expect(response.status()).toBe(400);
    await expect(room).toContainText("Target changed externally");
    expect(await current()).toBe(20.5);
    await expect(normal).toBeDisabled();
    await expect(boosted).toBeDisabled();
    await toggle.uncheck();
    await expect(normal).toHaveValue("20.5");
    await expect(boosted).toHaveValue("23");
    expect(await current()).toBe(20.5);
    for (const targets of [
      { normal: 4, boost: 6 },
      { normal: 21, boost: 20 },
      { normal: 21.05, boost: 23 },
      { normal: 21, boost: 27 },
    ]) {
      expect(
        (
          await request.post("/api/loadpoints/1/thermal/spaceHeating/targets", { data: targets })
        ).status()
      ).toBe(400);
    }
    expect(await current()).toBe(20.5);
    await page.setViewportSize({ width: 390, height: 1000 });
    await expect(normal).toBeEnabled();
    await normal.scrollIntoViewIfNeeded();
    const mobileBounds = await normal.boundingBox();
    if (!mobileBounds) throw new Error("Normal handle unavailable on mobile");
    const touchX = (value: number) =>
      mobileBounds.x + 10 + ((value - 5) / 25) * (mobileBounds.width - 20);
    const touch = await page.context().newCDPSession(page);
    await touch.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ x: touchX(20.5), y: mobileBounds.y + mobileBounds.height / 2 }],
    });
    await touch.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ x: touchX(18.5), y: mobileBounds.y + mobileBounds.height / 2 }],
    });
    await touch.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ x: touchX(19.5), y: mobileBounds.y + mobileBounds.height / 2 }],
    });
    await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await touch.detach();
    await expect(normal).toHaveValue("19.5");
    await expect.poll(current).toBe(19.5);
    await expect(boosted).toHaveValue("23");
  });
});
