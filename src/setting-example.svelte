<script lang="ts">
    import SettingPanel from "./libs/setting-panel.svelte";

    let groups: string[] = ["🌈 Default"];
    let focusGroup = groups[0];

    const SettingItems: ISettingItem[] = [
        {
            type: 'checkbox',
            title: 'checkbox',
            description: 'checkbox',
            key: 'a',
            value: true
        },
        {
            type: 'textinput',
            title: 'text',
            description: 'This is a text',
            key: 'b',
            value: 'This is a text',
            placeholder: 'placeholder'
        },
        {
            type: 'select',
            title: 'select',
            description: 'select',
            key: 'c',
            value: 'x',
            options: {
                x: 'x',
                y: 'y',
                z: 'z'
            }
        },
        {
            type: 'slider',
            title: 'slider',
            description: 'slider',
            key: 'd',
            value: 50,
            slider: {
                min: 0,
                max: 100,
                step: 1
            }
        }
    ];

    /********** Events **********/
    interface ChangeEvent {
        group: string;
        key: string;
        value: any;
    }

    const onChanged = ({ detail }: CustomEvent<ChangeEvent>) => {
        if (detail.group === groups[0]) {
            // setting.set(detail.key, detail.value);
        }
    };
</script>

<div class="fn__flex-1 fn__flex config__panel">
    <ul class="b3-tab-bar b3-list b3-list--background">
        {#each groups as group}
            <li
                data-name="editor"
                class:b3-list-item--focus={group === focusGroup}
                class="b3-list-item"
                on:click={() => {
                    focusGroup = group;
                }}
                on:keydown={() => {}}
            >
                <span class="b3-list-item__text">{group}</span>
            </li>
        {/each}
    </ul>
    <div class="config__tab-wrap">
        <SettingPanel
            group={groups[0]}
            settingItems={SettingItems}
            display={focusGroup === groups[0]}
            on:changed={onChanged}
        >
            <div class="fn__flex b3-label">
                💡 This is our default settings.
            </div>
        </SettingPanel>
    </div>
</div>

<style lang="scss">
    .config__panel {
        height: 100%;
    }
    .config__panel > ul > li {
        padding-left: 1rem;
    }
</style>

