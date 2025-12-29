<script lang="ts">
 import { CameraStream, useConnectionStatus, useResourceNames } from '@viamrobotics/svelte-sdk';
 import { MachineConnectionEvent } from '@viamrobotics/sdk';

 function filter(cameras) {
   if (!cameras) {
     return [];
   }
   return cameras;
 }

 const partID = 'xxx';
 const connectionStatus = useConnectionStatus(() => partID);
 const isConnected = $derived(connectionStatus.current === MachineConnectionEvent.CONNECTED);
 
 function toggleFullscreen() {
   if (!document.fullscreenElement) {
     document.documentElement.requestFullscreen();
   } else {
     document.exitFullscreen();
   }
 }

 let cams = ["aftdeck1", "aftdeck2", "walkport", "walkstbd"];

 const urlParams = new URLSearchParams(window.location.search);
 var camString = urlParams.get("cams");
 if (camString != null && camString != "") {
   cams = camString.split(",");
   console.log("cams", cams);
 }

 
 // Calculate columns for a square-ish grid: ceil(sqrt(n))
 const columns = Math.ceil(Math.sqrt(cams.length));
 
 // Chunk cameras into rows
 function getRows(cameras: string[], cols: number): string[][] {
   const result: string[][] = [];
   for (let i = 0; i < cameras.length; i += cols) {
     result.push(cameras.slice(i, i + cols));
  }
   return result;
 }
 
 const rows = getRows(cams, columns);
 
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
