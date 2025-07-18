# OneBot TUI

A terminal-based QQ chat client for slacking off.

## Features

- A TUI (Terminal User Interface) to display messages directly in your terminal.
- A daemon process that connects to a OneBot v11 implementation (like NapCat).
- A separate controller CLI to switch chats and send messages from scripts or other terminals.

## How to Use

1.  **Run the daemon:**
    ```sh
    ./onebot-tui-daemon
    ```
    On the first run, it will guide you to create a `config.yml` file.

2.  **Use the controller (in another terminal):**
    - **List chats:**
      ```sh
      ./onebot-tui-controller list
      ```
    - **Switch to a chat:**
      ```sh
      ./onebot-tui-controller use <CHAT_ID>
      ```
    - **Send a message:**
      ```sh
      ./onebot-tui-controller send <YOUR_MESSAGE>
      ```
