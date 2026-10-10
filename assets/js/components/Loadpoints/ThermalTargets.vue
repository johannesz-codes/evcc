<template>
	<section :aria-label="$t('main.thermal.spaceHeating')" class="mt-2">
		<div class="small text-muted mb-2">{{ $t("main.thermal.spaceHeating") }}</div>
		<ThermalRange
			v-if="
				ready && fn.range && fn.normalTarget !== undefined && fn.boostTarget !== undefined
			"
			ref="targets"
			:id="`thermal-${id}-spaceHeating-targets`"
			:normal-target="fn.normalTarget"
			:boost-target="fn.boostTarget"
			:temperature="fn.temperature"
			:min="Math.max(15, fn.range.min)"
			:max="Math.min(30, fn.range.max)"
			:step="fn.range.step"
			:disabled="disabled"
			:loading="busy"
			@change="setTargets"
		/>
		<p v-else class="small text-muted mb-0">{{ $t("main.thermal.unavailable") }}</p>
	</section>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import type { ThermalFunction } from "@/types/evcc";
import ThermalRange from "./ThermalRange.vue";

export default defineComponent({
	name: "ThermalTargets",
	components: { ThermalRange },
	props: {
		id: { type: String, required: true },
		fn: { type: Object as PropType<ThermalFunction>, required: true },
	},
	data() {
		return { busy: false };
	},
	computed: {
		ready() {
			return (
				this.fn.range && Math.min(30, this.fn.range.max) > Math.max(15, this.fn.range.min)
			);
		},
		disabled() {
			return (
				this.busy ||
				!this.fn.available ||
				this.fn.ownershipLost ||
				this.fn.pendingRestore ||
				this.fn.pendingTargetUpdate
			);
		},
	},
	methods: {
		async setTargets(targets: { normal: number; boost: number }) {
			this.busy = true;
			try {
				await api.post(`loadpoints/${this.id}/thermal/spaceHeating/targets`, targets);
			} catch {
				const range = this.$refs["targets"] as unknown as
					| InstanceType<typeof ThermalRange>
					| undefined;
				range?.reset();
			} finally {
				this.busy = false;
			}
		},
	},
});
</script>
