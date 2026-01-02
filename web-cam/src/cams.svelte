<script lang="ts">
 import { CameraStream, useConnectionStatus, useRobotClient } from '@viamrobotics/svelte-sdk';
 import { MachineConnectionEvent, CameraClient } from '@viamrobotics/sdk';

 const partID = 'xxx';
 const connectionStatus = useConnectionStatus(() => partID);
 const isConnected = $derived(connectionStatus.current === MachineConnectionEvent.CONNECTED);
 const robotClient = useRobotClient(() => partID);

 function toggleFullscreen() {
   if (!document.fullscreenElement) {
     document.documentElement.requestFullscreen();
   } else {
     document.exitFullscreen();
   }
 }

 const urlParams = new URLSearchParams(window.location.search);
 const camString = urlParams.get("cams");
 const filterOne = urlParams.get("filter-one") == "t";

 let initialCams = ["aftdeck1", "aftdeck2", "walkport", "walkstbd"];
 if (camString != null && camString != "") {
   initialCams = camString.split(",");
   console.log("cams", initialCams);
 }

 let cams = $state(initialCams);
 let filteringDone = $state(!filterOne);

 async function camGood(n) {
   // get the camera object
   const client = robotClient.current;
   if (!client) {
     console.log(`Camera ${n}: no robot client`);
     return true;
   }
   const camera = new CameraClient(client, n + "-filtered");

   try {
     const result = await camera.doCommand({ command: 'get' });
     console.log(`Camera ${n} result:`, result);
     return result.accepted.seconds_since < 120;
   } catch (e) {
     console.log(`Camera ${n} error:`, e);
     return false;
   }
 }

 async function filterCams() {
   let remaining = [...cams];
   while (remaining.length > 1) {
     const good = await camGood(remaining[0]);
     if (good) {
       cams = [remaining[0]];
       break;
     }
     remaining = remaining.slice(1);
   }
   if (remaining.length === 1) {
     cams = remaining;
   }
   filteringDone = true;
 }

 $effect(() => {
   if (isConnected && filterOne && !filteringDone) {
     filterCams();
   }
 });

 // Calculate columns for a square-ish grid: ceil(sqrt(n))
 const columns = $derived(Math.ceil(Math.sqrt(cams.length)));

 // Chunk cameras into rows
 function getRows(cameras: string[], cols: number): string[][] {
   const result: string[][] = [];
   for (let i = 0; i < cameras.length; i += cols) {
     result.push(cameras.slice(i, i + cols));
   }
   return result;
 }

 const rows = $derived(getRows(cams, columns));
 
</script>

{#if !isConnected}
  <p>Connecting...</p>
{/if}
{#if isConnected}
<button class="fullscreen-btn" onclick={toggleFullscreen}>Fullscreen</button>
<table border="0">
  <tbody>
    {#each rows as row}
    <tr>
      {#each row as cam}
      <td>
        <CameraStream partID="xxx" name={cam} width="100%" />
      </td>
      {/each}
    </tr>
    {/each}
  </tbody>
</table>
{/if}

<style>
  .fullscreen-btn {
    position: fixed;
    top: 10px;
    right: 10px;
    padding: 8px 16px;
    background: #333;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    z-index: 1000;
  }
  .fullscreen-btn:hover {
    background: #555;
  }
</style>
