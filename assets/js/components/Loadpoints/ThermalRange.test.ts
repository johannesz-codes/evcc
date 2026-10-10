import { mount } from "@vue/test-utils";
import { describe, expect, test } from "vite-plus/test";
import ThermalRange from "./ThermalRange.vue";

const render = () =>
  mount(ThermalRange, {
    props: { id: "room", normalTarget: 21, boostTarget: 22, min: 15, max: 30, step: 0.1 },
    global: { mocks: { $t: (key: string) => key } },
  });

describe("thermal range", () => {
  test("handles cannot cross or exceed 5 K", async () => {
    const wrapper = render();
    const inputs = wrapper.findAll<HTMLInputElement>("[type=range]");
    await inputs[0]!.setValue(30);
    expect(wrapper.emitted("change")![0]).toEqual([{ normal: 21.9, boost: 22 }]);
    await inputs[1]!.setValue(15);
    expect(wrapper.emitted("change")![1]).toEqual([{ normal: 21.9, boost: 22 }]);
    await inputs[0]!.setValue(15);
    expect(wrapper.emitted("change")![2]).toEqual([{ normal: 17, boost: 22 }]);
  });

  test("device updates do not interrupt a drag", async () => {
    const wrapper = render();
    const input = wrapper.findAll<HTMLInputElement>("[type=range]")[0]!;
    input.element.value = "20.5";
    await input.trigger("input");
    await wrapper.setProps({ normalTarget: 21.2, boostTarget: 22.2 });
    expect(input.element.value).toBe("20.5");
    expect(wrapper.emitted("change")).toBeUndefined();
    await input.trigger("change");
    expect(wrapper.emitted("change")![0]).toEqual([{ normal: 20.5, boost: 22 }]);
    await wrapper.setProps({ disabled: true });
    await wrapper.setProps({ disabled: false });
    expect(input.element.value).toBe("21.2");
  });

  test("normal target at the device maximum can be reduced", async () => {
    const wrapper = render();
    await wrapper.setProps({ normalTarget: 30, boostTarget: 31 });
    await wrapper.findAll<HTMLInputElement>("[type=range]")[0]!.setValue(29);
    expect(wrapper.emitted("change")![0]).toEqual([{ normal: 29, boost: 30 }]);
  });
  test("an external target outside the UI range is never written on mount", async () => {
    const wrapper = render();
    await wrapper.setProps({ normalTarget: 12, boostTarget: 13 });
    expect(wrapper.text()).toContain("12.0°C");
    expect(wrapper.emitted("change")).toBeUndefined();
    await wrapper.findAll<HTMLInputElement>("[type=range]")[1]!.setValue(17);
    expect(wrapper.emitted("change")![0]).toEqual([{ normal: 15, boost: 17 }]);
  });
});
