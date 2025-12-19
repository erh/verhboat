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
</script>

{#if !isConnected}
  <p>Connecting...</p>
{/if}
{#if isConnected}
<button class="fullscreen-btn" onclick={toggleFullscreen}>Fullscreen</button>
<table border="0">
  <tbody>
    <tr>
      <td>
        <CameraStream partID="xxx" name="aftdeck1" width="100%" />
      </td>
      <td>
        <CameraStream partID="xxx" name="aftdeck2" width="100%" />
      </td>
    </tr>
    <tr>
      <td>
        <CameraStream partID="xxx" name="walkport" width="100%"/>
      </td>
      <td>
        <CameraStream partID="xxx" name="walkstbd" width="100%"/>
      </td>
    </tr>
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
