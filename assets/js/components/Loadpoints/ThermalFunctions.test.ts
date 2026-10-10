import { mount, flushPromises } from "@vue/test-utils";
import { describe, expect, test, vi, beforeEach } from "vite-plus/test";
import ThermalFunctions from "./ThermalFunctions.vue";
import type { ThermalFunction } from "@/types/evcc";
import api from "@/api";
import en from "../../../../i18n/en.json";

vi.mock("@/api", () => ({ default: { post: vi.fn() } }));

const lookup = (key: string): string => {
  const value = key.split(".").reduce<any>((obj, part) => obj?.[part], en);
  return typeof value === "string" ? value : key;
};

const functions = (): ThermalFunction[] => [
  {
    kind: "spaceHeating",
    boost: { delta: 1 },
    target: 21,
    requested: false,
    active: false,
    available: true,
    ownershipLost: false,
    pendingRestore: false,
  },
  {
    kind: "domesticHotWater",
    boost: { mode: "large" },
    mode: "medium",
    boostModes: ["medium", "large"],
    requested: false,
    active: false,
    available: true,
    ownershipLost: false,
    pendingRestore: false,
  },
];

const render = (states = functions()) =>
  mount(ThermalFunctions, {
    props: { id: "1", functions: states },
    global: { mocks: { $t: lookup } },
  });

beforeEach(() => {
  vi.mocked(api.post).mockReset();
});

describe("thermal controls", () => {
  test("shared plant and independent targets", () => {
    const wrapper = render();
    expect(wrapper.text()).toContain("Power and energy are measured together");
    expect(wrapper.text()).toContain("21.0 °C");
    expect(wrapper.text()).toContain("Medium");
    expect(wrapper.findAll("[role=switch]")).toHaveLength(2);
    expect(wrapper.findAll("option").map((option) => option.text())).not.toContain("Not in use");
  });

  test("only the requested function is busy", async () => {
    let complete!: (value: any) => void;
    vi.mocked(api.post).mockImplementation(
      () =>
        new Promise((resolve) => {
          complete = resolve;
        })
    );
    const wrapper = render();
    const switches = wrapper.findAll<HTMLInputElement>("[role=switch]");
    await switches[0]!.setValue(true);
    expect(api.post).toHaveBeenCalledWith("loadpoints/1/thermal/spaceHeating/boost/true");
    expect(switches[0]!.element.disabled).toBe(true);
    expect(switches[1]!.element.disabled).toBe(false);
    complete({});
    await flushPromises();
    expect(switches[0]!.element.disabled).toBe(false);
  });

  test("failed request restores the displayed switch", async () => {
    vi.mocked(api.post).mockRejectedValue(new Error("offline"));
    const wrapper = render();
    const input = wrapper.find<HTMLInputElement>("[role=switch]");
    await input.setValue(true);
    await flushPromises();
    expect(input.element.checked).toBe(false);
    expect(input.element.disabled).toBe(false);
  });

  test("lost ownership and pending restoration", () => {
    const states = functions();
    states[0] = { ...states[0]!, requested: true, ownershipLost: true };
    states[1] = { ...states[1]!, available: false, pendingRestore: true };
    const wrapper = render(states);
    expect(wrapper.text()).toContain("Target changed externally");
    expect(wrapper.text()).toContain("Restoration pending");
    expect(
      wrapper.findAll<HTMLSelectElement>("select").every((select) => select.element.disabled)
    ).toBe(true);
    expect(wrapper.text()).not.toContain("Boost active");
  });
});
