// The module 'vscode' contains the VS Code extensibility API
// Import the module and reference it with the alias vscode in your code below
import * as vscode from 'vscode';
import * as fs from 'fs';

/**
 * Manages the webview panel that displays the goasm ui 
 */
class GoASMViewProvider {
	/**
	 * Track the currently panel. Only allow a single panel to exist at a time.
	 */
	public static currentPanel: GoASMViewProvider | undefined;

	private static readonly viewType = 'goasm';

	private readonly _panel: vscode.WebviewPanel;
	private readonly _extensionUri: vscode.Uri;
	private _disposables: vscode.Disposable[] = [];

	public static createOrShow(extensionContext: vscode.ExtensionContext) {
		const column = vscode.window.activeTextEditor
			? vscode.window.activeTextEditor.viewColumn
			: undefined;

		// If we already have a panel, show it.
		if (GoASMViewProvider.currentPanel) {
			GoASMViewProvider.currentPanel._panel.reveal(column);
			return;
		}

		// Otherwise, create a new panel.
		const panel = vscode.window.createWebviewPanel(
			GoASMViewProvider.viewType,
			'Go Assembly',
			column || vscode.ViewColumn.One,
			{
				// Enable JavaScript in the webview
				enableScripts: true,

				// And restrict the webview to only loading content from our extension's directory.
				localResourceRoots: [
					vscode.Uri.joinPath(extensionContext.extensionUri, 'out')
				]
			}
		);

		GoASMViewProvider.currentPanel = new GoASMViewProvider(panel, extensionContext.extensionUri);
	}

	private constructor(panel: vscode.WebviewPanel, extensionUri: vscode.Uri) {
		this._panel = panel;
		this._extensionUri = extensionUri;

		// Set the webview's initial html content
		this._update();

		// Listen for when the panel is disposed
		// This happens when the user closes the panel or when the panel is closed programmatically
		this._panel.onDidDispose(() => this.dispose(), null, this._disposables);

		// Update the content based on view changes
		this._panel.onDidChangeViewState(
			e => {
				if (this._panel.visible) {
					this._update();
				}
			},
			null,
			this._disposables
		);

		// Listen for configuration changes
		vscode.workspace.onDidChangeConfiguration(
			e => {
				if (e.affectsConfiguration('goasm.serverUrl')) {
					this._update();
				}
			},
			null,
			this._disposables
		);

		// Handle messages from the webview
		this._panel.webview.onDidReceiveMessage(
			message => {
				switch (message.command) {
					case 'ready':
						// Webview is ready to receive messages
						console.log('Webview goasm is ready');
						break;
				}
			},
			null,
			this._disposables
		);
	}

	public dispose() {
		GoASMViewProvider.currentPanel = undefined;

		// Clean up our resources
		this._panel.dispose();

		while (this._disposables.length) {
			const x = this._disposables.pop();
			if (x) {
				x.dispose();
			}
		}
	}

	private _update() {
		const webview = this._panel.webview;
		this._panel.title = "Go Assembly";
		this._panel.webview.html = this._getHtmlForWebview(webview);
	}

	private _getHtmlForWebview(webview: vscode.Webview): string {
		// Get the local path to main script
		// const iconsUri = webview.asWebviewUri(
		const mainAsm = webview.asWebviewUri(
			vscode.Uri.joinPath(this._extensionUri, 'out', 'webview', 'main.wasm'),
		)

		const scriptPathOnDisk = vscode.Uri.joinPath(this._extensionUri, 'media', 'index.html');

		// Content security policy
		const nonce = this._getNonce();
		const serverUrl = vscode.workspace.getConfiguration('goasm').get('serverUrl', 'http://localhost:8080');

		// Read the HTML file directly or provide a fallback
		try {
			// This may not work in all environments due to extension packaging
			if (fs.existsSync(scriptPathOnDisk.fsPath)) {
				let htmlContent = fs.readFileSync(scriptPathOnDisk.fsPath, 'utf8');

				// Add CSP and nonce to the HTML content
				htmlContent = htmlContent.replace(
					'<head>',
					`<head>
					<meta http-equiv="Content-Security-Policy" content="default-src 'none'; connect-src ${webview.cspSource} ${serverUrl}; img-src ${webview.cspSource}; style-src ${webview.cspSource} 'unsafe-inline'; script-src 'nonce-${nonce}' 'unsafe-eval'; worker-src blob:;">`
				).replace(
					'<script>',
					`<script nonce="${nonce}">`
				).replace("{{ WASM_MAIN }}", mainAsm.toString());
				
				// Update the server URL in the Go wasm client
				htmlContent = htmlContent.replace(
					'this.argv = ["js", "-client", "-dark"];',
					`this.argv = ["js", "-client", "-dark", "-addr", "${serverUrl}"];`
				);
				
				return htmlContent;
			}
		} catch (error) {
			console.error(`Failed to read HTML file: ${error}`);
		}

		// If we couldn't load the file or it doesn't exist, provide a fallback HTML content
		return `<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<meta http-equiv="Content-Security-Policy" content="default-src 'none'; connect-src ${webview.cspSource} ${serverUrl}; img-src ${webview.cspSource}; style-src 'unsafe-inline'; script-src 'nonce-${nonce}' 'unsafe-eval'; worker-src blob:;">
			<title>Go Assembly</title>
		</head>
		<body>
		<h1>Go Assembly Failed</h1>
		</body>
		</html>`;
	}

	/**
	 * Generate a nonce string for CSP
	 */
	private _getNonce(): string {
		let text = '';
		const possible = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
		for (let i = 0; i < 32; i++) {
			text += possible.charAt(Math.floor(Math.random() * possible.length));
		}
		return text;
	}
}

// This method is called when your extension is activated
// Your extension is activated the very first time the command is executed
export function activate(context: vscode.ExtensionContext) {
	// Use the console to output diagnostic information (console.log) and errors (console.error)
	// This line of code will only be executed once when your extension is activated
	console.log('Congratulations, your extension "goasm" is now active!');

	// The command has been defined in the package.json file
	// Now provide the implementation of the command with registerCommand
	// The commandId parameter must match the command field in package.json
	const helloDisposable = vscode.commands.registerCommand('goasm.helloWorld', () => {
		// The code you place here will be executed every time your command is executed
		// Display a message box to the user
		vscode.window.showInformationMessage('Hello World from goasm!');
	});

	// Register the command to show the goasm webview
	const goasmDisposable = vscode.commands.registerCommand('goasm.showAsm', () => {
		GoASMViewProvider.createOrShow(context);
	});

	context.subscriptions.push(helloDisposable);
	context.subscriptions.push(goasmDisposable);
}

// This method is called when your extension is deactivated
export function deactivate() { }