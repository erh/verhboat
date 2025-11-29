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
</script>

{#if !isConnected}
  <p>Connecting...</p>
{/if}
{#if isConnected}
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
