<template>
	<button
		type="button"
		class="btn btn-link text-decoration-none text-start evcc-default-text keyboard-focus-ring p-0 mb-3 d-flex align-items-center justify-content-between gap-2"
		:aria-label="$t('main.thermal.settings')"
		@click="$emit('open-settings')"
	>
		<span class="d-flex flex-wrap gap-2 column-gap-4 small" aria-live="polite">
			<span v-for="fn in summaries" :key="fn.kind">
				<span class="text-muted">{{ fn.label }}</span>
				<strong class="ms-1" :class="{ 'text-evcc': fn.active }">{{ fn.target }}</strong>
				<span v-if="fn.status" class="ms-1" :class="fn.statusClass">{{ fn.status }}</span>
			</span>
		</span>
		<shopicon-regular-adjust size="s" class="evcc-gray flex-shrink-0" />
	</button>
</template>

<script lang="ts">
import "@h2d2/shopicons/es/regular/adjust";
import { defineComponent, type PropType } from "vue";
import type { ThermalFunction } from "@/types/evcc";
import formatter from "@/mixins/formatter";

export default defineComponent({
	name: "ThermalSummary",
	mixins: [formatter],
	props: {
		functions: { type: Array as PropType<ThermalFunction[]>, required: true },
	},
	emits: ["open-settings"],
	computed: {
		summaries() {
			return this.functions.map((fn) => {
				const status = this.statusKey(fn);
				return {
					kind: fn.kind,
					label: this.$t(`main.thermal.${fn.kind}`),
					target:
						fn.target !== undefined
							? this.fmtTemperature(fn.target)
							: fn.mode
								? this.$t(`main.thermal.modes.${fn.mode}`)
								: "",
					active: fn.active,
					status: status && this.$t(`main.thermal.${status}`),
					statusClass: fn.active ? "text-evcc" : "text-warning",
				};
			});
		},
	},
	methods: {
		statusKey(fn: ThermalFunction) {
			if (fn.ownershipLost) return "changed";
			if (fn.pendingRestore) return "restoring";
			if (fn.pendingTargetUpdate) return "updating";
			if (!fn.available) return "unavailable";
			if (fn.active) return "active";
			if (fn.requested) return "requested";
			return undefined;
		},
	},
});
</script>
