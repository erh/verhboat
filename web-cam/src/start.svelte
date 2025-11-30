<script lang="ts">
 import { ViamProvider } from '@viamrobotics/svelte-sdk';
 import type { DialConf } from '@viamrobotics/sdk';
 import Cams from "./cams.svelte";

 import type { Snippet } from 'svelte';

 interface Props {
   children?: Snippet;
 }
 let { children }: Props = $props();

 const urlParams = new URLSearchParams(window.location.search);

 var host = urlParams.get("host");
 var apiKey = urlParams.get("api-key");
 var authEntity = urlParams.get("authEntity");
 
 if (!host || host == "") {
   host = getCookie("host");
   apiKey = getCookie("api-key");
   authEntity = getCookie("api-key-id");
 }
 
 const dialConfigs = {
   'xxx': {
     host: host,
     credentials: {
       "type": 'api-key',
       payload: apiKey,
       authEntity: authEntity,
     },
     signalingAddress: 'https://app.viam.com:443',
     disableSessions: false,
   },
 };
</script>

<ViamProvider {dialConfigs}>
  <Cams />
  {#if children}
    {@render children()}
  {/if}
</ViamProvider>
