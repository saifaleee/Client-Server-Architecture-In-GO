import subprocess
import time
import os
import sys

def clear_screen():
    os.system('cls' if os.name == 'nt' else 'clear')

def print_header():
    clear_screen()
    print("=" * 50)
    print("Distributed Matrix Operations System Runner")
    print("=" * 50)
    print()

def prompt_continue():
    while True:
        response = input("\nDo you want to continue? (y/n): ").lower()
        if response in ['y', 'n']:
            return response == 'y'

def run_command(command, window_title):
    if os.name == 'nt':  # Windows
        return subprocess.Popen(['start', 'cmd', '/k', f'title {window_title} && {command}'], shell=True)
    else:  # Unix/Linux/MacOS
        return subprocess.Popen(['gnome-terminal', '--title', window_title, '--', 'bash', '-c', f'{command}; exec bash'])

def setup_go_modules():
    print("Setting up Go modules...")
    if not os.path.exists("go.mod"):
        subprocess.run(["go", "mod", "init", "matrix-operations"])
        subprocess.run(["go", "mod", "tidy"])
    print("Go modules setup complete!")

def main():
    processes = []
    
    try:
        # Setup Go modules first
        setup_go_modules()
        
        while True:
            print_header()
            print("System Components Status:")
            print("1. Coordinator: ", "❌ Not Running" if not processes else "✅ Running")
            print("2. Workers:    ", f"✅ {len(processes)-1} Running" if len(processes) > 1 else "❌ Not Running")
            print("3. Client:     ", "❌ Not Started" if len(processes) <= 3 else "✅ Complete")
            print("\nNext steps:")

            if not processes:
                print("\nStarting Coordinator...")
                if prompt_continue():
                    processes.append(run_command('cd coordinator && go run main.go', 'Coordinator'))
                    print("Coordinator started! Waiting for initialization...")
                    time.sleep(2)
                else:
                    break

            elif len(processes) < 4:  # Start 3 workers
                worker_num = len(processes)
                print(f"\nStarting Worker {worker_num}...")
                if prompt_continue():
                    processes.append(run_command(f'cd worker && go run main.go -id worker{worker_num}', f'Worker {worker_num}'))
                    print(f"Worker {worker_num} started!")
                    time.sleep(2)
                else:
                    break

            elif len(processes) == 4:
                print("\nStarting Client...")
                if prompt_continue():
                    processes.append(run_command('cd client && go run main.go', 'Client'))
                    print("Client started!")
                    print("\nAll components are now running!")
                    break
                else:
                    break

            else:
                break

        print("\nSystem is running! Press Ctrl+C to shut down all components.")
        while True:
            time.sleep(1)

    except KeyboardInterrupt:
        print("\nShutting down all components...")
        for process in processes:
            try:
                process.terminate()
            except:
                pass
        print("System shutdown complete!")

if __name__ == "__main__":
    main()