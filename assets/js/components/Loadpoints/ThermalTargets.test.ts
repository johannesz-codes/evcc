import { mount, flushPromises } from "@vue/test-utils";
import { describe, expect, test, vi, beforeEach } from "vite-plus/test";
import ThermalTargets from "./ThermalTargets.vue";
import type { ThermalFunction } from "@/types/evcc";
import api from "@/api";

vi.mock("@/api", () => ({ default: { post: vi.fn() } }));
const functions = (): ThermalFunction[] => [
  {
    kind: "spaceHeating",
    boost: { delta: 1 },
    target: 21,
    normalTarget: 21,
    boostTarget: 22,
    temperature: 20,
    range: { min: 5, max: 30, step: 0.1 },
    requested: false,
    active: false,
    available: true,
    ownershipLost: false,
    pendingRestore: false,
  },
];
const render = (states = functions()) =>
  mount(ThermalTargets, {
    props: { id: "1", fn: states[0]! },
    global: { mocks: { $t: (key: string) => key } },
  });
beforeEach(() => {
  vi.mocked(api.post).mockReset();
});
describe("main card room targets", () => {
  test("normal handle submits both temperatures", async () => {
    vi.mocked(api.post).mockResolvedValue({});
    const wrapper = render();
    await wrapper.findAll<HTMLInputElement>("[type=range]")[0]!.setValue(20.5);
    expect(api.post).toHaveBeenCalledWith("loadpoints/1/thermal/spaceHeating/targets", {
      normal: 20.5,
      boost: 22,
    });
  });

  test("both handles remain editable during an owned boost", () => {
    const states = functions();
    states[0] = { ...states[0]!, requested: true, active: true, target: 22 };
    const wrapper = render(states);
    expect(
      wrapper.findAll<HTMLInputElement>("[type=range]").every((input) => !input.element.disabled)
    ).toBe(true);
  });

  test("failed temperature request restores both handles", async () => {
    vi.mocked(api.post).mockRejectedValue(new Error("offline"));
    const wrapper = render();
    const inputs = wrapper.findAll<HTMLInputElement>("[type=range]");
    await inputs[0]!.setValue(20.5);
    await flushPromises();
    expect(inputs[0]!.element.value).toBe("21");
    expect(inputs[1]!.element.value).toBe("22");
    expect(inputs[0]!.element.disabled).toBe(false);
  });

  test("successive edits retain the latest normal target before state arrives", async () => {
    vi.mocked(api.post).mockResolvedValue({});
    const wrapper = render();
    const inputs = wrapper.findAll<HTMLInputElement>("[type=range]");
    await inputs[0]!.setValue(20.5);
    await flushPromises();
    await inputs[1]!.setValue(22.5);
    expect(api.post).toHaveBeenLastCalledWith("loadpoints/1/thermal/spaceHeating/targets", {
      normal: 20.5,
      boost: 22.5,
    });
  });
  test("room sensor reading and 15 to 30 scale", () => {
    const wrapper = render();
    expect(wrapper.text()).toContain("20.0°C");
    for (const input of wrapper.findAll("[type=range]")) {
      expect(input.attributes("min")).toBe("15");
      expect(input.attributes("max")).toBe("30");
    }
    expect(api.post).not.toHaveBeenCalled();
  });
  test.each(["ownershipLost", "pendingRestore", "pendingTargetUpdate"])(
    "%s disables edits",
    (flag) => {
      const wrapper = render([{ ...functions()[0]!, [flag]: true }]);
      expect(
        wrapper.findAll<HTMLInputElement>("[type=range]").every((input) => input.element.disabled)
      ).toBe(true);
    }
  );
  test("missing measurement leaves targets editable", () => {
    const wrapper = render([{ ...functions()[0]!, temperature: undefined }]);
    expect(wrapper.text()).not.toContain("20.0°C");
    expect(
      wrapper.findAll<HTMLInputElement>("[type=range]").every((input) => !input.element.disabled)
    ).toBe(true);
  });
});
