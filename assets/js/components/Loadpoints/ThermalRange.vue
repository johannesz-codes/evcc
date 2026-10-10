<template>
	<div :class="{ 'opacity-50': disabled }">
		<div class="d-flex justify-content-between gap-3 mb-1 small">
			<label :for="`${id}-normal`">
				{{ $t("main.thermal.normalTarget") }}
				<strong class="d-block">{{ fmtTemperature(normal) }}</strong>
			</label>
			<label :for="`${id}-boost`" class="text-end">
				{{ $t("main.thermal.boostTarget") }}
				<strong class="d-block text-evcc">{{ fmtTemperature(boost) }}</strong>
			</label>
		</div>
		<div
			class="thermal-range"
			:style="trackStyle"
			role="group"
			:aria-label="$t('main.thermal.temperatures')"
		>
			<div class="range-track"></div>
			<div class="range-selection"></div>
			<input
				:id="`${id}-normal`"
				class="form-range normal-range"
				type="range"
				:min="min"
				:max="max"
				:step="step"
				:value="normal"
				:disabled="disabled"
				:aria-label="$t('main.thermal.normalTarget')"
				:aria-valuemin="normalMin"
				:aria-valuemax="normalMax"
				:aria-valuetext="fmtTemperature(normal)"
				@input="preview('normal', $event)"
				@change="commit"
			/>
			<input
				:id="`${id}-boost`"
				class="form-range boost-range"
				type="range"
				:min="min"
				:max="max"
				:step="step"
				:value="boost"
				:disabled="disabled"
				:aria-label="$t('main.thermal.boostTarget')"
				:aria-valuemin="boostMin"
				:aria-valuemax="boostMax"
				:aria-valuetext="fmtTemperature(boost)"
				@input="preview('boost', $event)"
				@change="commit"
			/>
		</div>
		<div class="d-flex justify-content-between small text-muted" aria-hidden="true">
			<span>{{ fmtTemperature(min) }}</span>
			<span>{{ fmtTemperature(max) }}</span>
		</div>
	</div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import formatter from "@/mixins/formatter";

export default defineComponent({
	name: "ThermalRange",
	mixins: [formatter],
	props: {
		id: { type: String, required: true },
		normalTarget: { type: Number, required: true },
		boostTarget: { type: Number, required: true },
		min: { type: Number, required: true },
		max: { type: Number, required: true },
		step: { type: Number, required: true },
		disabled: Boolean,
		loading: Boolean,
	},
	emits: ["change"],
	data() {
		return {
			normal: this.normalTarget,
			boost: Math.min(this.max, this.boostTarget),
			editing: false,
		};
	},
	computed: {
		normalMin() {
			return Math.max(this.min, this.boost - 5);
		},
		normalMax() {
			return Math.min(this.max - this.step, this.boost - this.step);
		},
		boostMin() {
			return Math.max(this.min + this.step, this.normal + this.step);
		},
		boostMax() {
			return Math.min(this.max, this.normal + 5);
		},
		trackStyle() {
			return {
				"--normal-position": (this.normal - this.min) / (this.max - this.min),
				"--boost-position": (this.boost - this.min) / (this.max - this.min),
			};
		},
	},
	watch: {
		normalTarget() {
			if (!this.editing) this.reset();
		},
		boostTarget() {
			if (!this.editing) this.reset();
		},
		disabled(value) {
			this.editing = false;
			if (value && !this.loading) this.reset();
		},
	},
	methods: {
		reset() {
			this.normal = this.normalTarget;
			this.boost = Math.min(this.max, this.boostTarget);
			this.editing = false;
		},
		preview(handle: "normal" | "boost", event: Event) {
			if (this.disabled) return;
			const input = event.target as HTMLInputElement;
			const lower = handle === "normal" ? this.normalMin : this.boostMin;
			const upper = handle === "normal" ? this.normalMax : this.boostMax;
			const value = Math.min(upper, Math.max(lower, Number(input.value)));
			this[handle] = Number((Math.round(value / this.step) * this.step).toPrecision(12));
			input.value = String(this[handle]);
			this.editing = true;
		},
		commit() {
			if (this.disabled || !this.editing) return;
			this.editing = false;
			this.$emit("change", { normal: this.normal, boost: this.boost });
		},
	},
});
</script>

<style scoped>
.thermal-range {
	position: relative;
	height: 44px;
}
.range-track,
.range-selection {
	position: absolute;
	top: 20px;
	height: 4px;
	border-radius: 2px;
}
.range-track {
	left: 10px;
	right: 10px;
	background: var(--bs-secondary-bg);
}
.range-selection {
	left: calc(10px + (100% - 20px) * var(--normal-position));
	width: max(0px, calc((100% - 20px) * (var(--boost-position) - var(--normal-position))));
	background: var(--evcc-green);
}
.form-range {
	--thumb-size: 20px;
	--thumb-ring: 3px;
	--track-height: 4px;
	background: none;
	position: absolute;
	left: 0;
	top: 10px;
	height: 24px;
	margin: 0;
	pointer-events: none;
	touch-action: none;
}
.form-range::-webkit-slider-runnable-track {
	background: transparent;
}
.form-range::-moz-range-track {
	background: transparent;
}
.form-range::-webkit-slider-thumb {
	width: 20px;
	height: 20px;
	margin-top: -8px;
	pointer-events: auto;
}
.form-range::-moz-range-thumb {
	width: 20px;
	height: 20px;
	pointer-events: auto;
}
.normal-range::-webkit-slider-thumb {
	background: var(--evcc-default-text);
}
.normal-range::-moz-range-thumb {
	background: var(--evcc-default-text);
}
.normal-range {
	z-index: 1;
}
.form-range:focus {
	z-index: 2;
}
.form-range:disabled::-webkit-slider-thumb {
	pointer-events: none;
}
.form-range:disabled::-moz-range-thumb {
	pointer-events: none;
}
</style>
