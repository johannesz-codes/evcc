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
			<div class="progress range-track">
				<div class="range-fill range-boost"></div>
				<div v-if="temperature !== undefined" class="range-fill range-current"></div>
			</div>
			<input
				:id="`${id}-normal`"
				class="slider normal-range"
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
				class="slider boost-range"
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
		<div class="d-flex justify-content-between gap-2 small text-muted">
			<span aria-hidden="true">{{ fmtTemperature(min) }}</span>
			<span v-if="temperature !== undefined">
				{{ $t("main.thermal.roomTemperature") }}
				<strong class="evcc-default-text ms-1">{{ fmtTemperature(temperature) }}</strong>
			</span>
			<span aria-hidden="true">{{ fmtTemperature(max) }}</span>
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
		temperature: Number,
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
				"--normal-position": this.position(this.normal),
				"--boost-position": this.position(this.boost),
				"--current-position": this.position(this.temperature ?? this.normal),
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
		position(value: number) {
			return Math.min(1, Math.max(0, (value - this.min) / (this.max - this.min)));
		},
		reset() {
			this.normal = this.normalTarget;
			this.boost = Math.min(this.max, this.boostTarget);
			this.editing = false;
		},
		preview(handle: "normal" | "boost", event: Event) {
			if (this.disabled) return;
			const input = event.target as HTMLInputElement;
			if (handle === "normal")
				this.boost = Math.min(this.max, Math.max(this.min + this.step, this.boost));
			else this.normal = Math.min(this.max - this.step, Math.max(this.min, this.normal));
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
	--height: 32px;
	--thumb-overlap: 6px;
	--thumb-width: 12px;
	position: relative;
	height: var(--height);
	margin: 10px 0;
	container-type: inline-size;
}
.range-track {
	position: absolute;
	inset: 0;
	height: 100%;
	background: var(--evcc-background);
}
.range-fill {
	position: absolute;
	inset: 0 auto 0 0;
	background: var(--evcc-heating-gradient);
	background-size: 500cqw 100%;
	background-position-x: calc(var(--current-position) * -400cqw);
	background-repeat: no-repeat;
}
.range-current {
	width: calc(var(--current-position) * 100%);
}
.range-boost {
	width: calc(var(--boost-position) * 100%);
	opacity: 0.5;
}
.slider {
	-webkit-appearance: none;
	appearance: none;
	position: absolute;
	left: 0;
	top: calc(var(--thumb-overlap) * -1);
	height: calc(100% + 2 * var(--thumb-overlap));
	width: 100%;
	margin: 0;
	background: transparent;
	pointer-events: none;
	touch-action: none;
}
.slider:focus {
	outline: none;
	z-index: 2;
}
.slider::-webkit-slider-runnable-track {
	background: transparent;
	border: none;
	height: 100%;
	margin: 0 -6px;
}
.slider::-moz-range-track {
	background: transparent;
	border: none;
	height: 100%;
	margin: 0 -6px;
}
.slider::-webkit-slider-thumb {
	-webkit-appearance: none;
	height: 100%;
	width: var(--thumb-width);
	border: none;
	border-radius: var(--thumb-overlap);
	background: var(--evcc-heating-gradient);
	background-size: 100cqw 100%;
	background-position-x: calc(var(--handle-position) * 100%);
	box-shadow: 0 0 6px var(--evcc-background);
	cursor: grab;
	pointer-events: auto;
}
.slider::-moz-range-thumb {
	height: 100%;
	width: var(--thumb-width);
	border: none;
	border-radius: var(--thumb-overlap);
	background: var(--evcc-heating-gradient);
	background-size: 100cqw 100%;
	background-position-x: calc(var(--handle-position) * 100%);
	box-shadow: 0 0 6px var(--evcc-background);
	cursor: grab;
	pointer-events: auto;
}
.normal-range {
	--handle-position: var(--normal-position);
	z-index: 1;
}
.boost-range {
	--handle-position: var(--boost-position);
}
.slider:focus-visible::-webkit-slider-thumb {
	outline: 2px solid var(--evcc-default-text);
	outline-offset: 2px;
}
.slider:focus-visible::-moz-range-thumb {
	outline: 2px solid var(--evcc-default-text);
	outline-offset: 2px;
}
.slider:active::-webkit-slider-thumb {
	cursor: grabbing;
}
.slider:active::-moz-range-thumb {
	cursor: grabbing;
}
.slider:disabled::-webkit-slider-thumb {
	pointer-events: none;
}
.slider:disabled::-moz-range-thumb {
	pointer-events: none;
}
</style>
