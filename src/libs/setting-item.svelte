<script lang="ts">
    import { createEventDispatcher } from "svelte";
    export let type: string; // Setting Type
    export let title: string; // Displayint Setting Title
    export let description: string; // Displaying Setting Text
    export let settingKey: string;
    export let settingValue: any;

    //Optional
    export let placeholder: string = ""; // Use it if type is input
    export let options: { [key: string | number]: string } = {}; // Use it if type is select
    export let slider: {
        min: number;
        max: number;
        step: number;
    } = { min: 0, max: 100, step: 1 }; // Use it if type is slider

    const dispatch = createEventDispatcher();

    function clicked() {
        dispatch("clicked");
    }

    function changed() {
        dispatch("changed", { key: settingKey, value: settingValue });
    }
</script>

<label class="fn__flex b3-label">
    <div class="fn__flex-1">
        {title}
        <div class="b3-label__text">
            {@html description}
        </div>
    </div>
    <span class="fn__space" />
    <!-- <slot /> -->
    {#if type === "checkbox"}
        <!-- Checkbox -->
        <input
            class="b3-switch fn__flex-center"
            id={settingKey}
            type="checkbox"
            bind:checked={settingValue}
            on:change={changed}
        />
    {:else if type === "textinput"}
        <!-- Text Input -->
        <input
            class="b3-text-field fn__flex-center fn__size200"
            id={settingKey}
            {placeholder}
            bind:value={settingValue}
            on:change={changed}
        />
    {:else if type === "number"}
        <input
            class="b3-text-field fn__flex-center fn__size200"
            id={settingKey}
            type="number"
            bind:value={settingValue}
            on:change={changed}
        />
    {:else if type === "button"}
        <!-- Button Input -->
        <button
            class="b3-button b3-button--outline fn__flex-center fn__size200"
            id={settingKey}
            on:click={clicked}
        >
            {settingValue}
        </button>
    {:else if type === "select"}
        <!-- Dropdown select -->
        <select
            class="b3-select fn__flex-center fn__size200"
            id="iconPosition"
            bind:value={settingValue}
            on:change={changed}
        >
            {#each Object.entries(options) as [value, text]}
                <option {value}>{text}</option>
            {/each}
        </select>
    {:else if type == "slider"}
        <!-- Slider -->
        <div class="b3-tooltips b3-tooltips__n" aria-label={settingValue}>
            <input
                class="b3-slider fn__size200"
                id="fontSize"
                min={slider.min}
                max={slider.max}
                step={slider.step}
                type="range"
                bind:value={settingValue}
                on:change={changed}
            />
        </div>
    {/if}
</label>
