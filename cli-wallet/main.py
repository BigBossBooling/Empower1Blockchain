import click
import os
from wallet import (
    generate_key_pair,
    save_private_key,
    load_private_key,
    public_key_to_address,
    get_public_key_bytes
)
from transaction import Transaction
from client import send_transaction

CONTEXT_SETTINGS = dict(help_option_names=['-h', '--help'])

@click.group(context_settings=CONTEXT_SETTINGS)
def cli():
    """Empower1 CLI Wallet"""
    pass

@cli.command("generate-wallet", short_help="Generate a new ECDSA keypair and save it.")
@click.option('--outfile', default="wallet.pem", help="Output file for the wallet.", show_default=True)
@click.option('--password', help="Password to encrypt the wallet file (optional).", default=None)
def generate_wallet_cmd(outfile, password):
    """Generates a new ECDSA (SECP256R1) keypair and saves it to a PEM file."""
    try:
        priv_key, pub_key = generate_key_pair()
        save_private_key(priv_key, outfile, password)
        address = public_key_to_address(pub_key)
        click.echo(f"New wallet generated and saved to: {outfile}")
        click.echo(f"Public Address: {address}")
    except Exception as e:
        click.echo(f"Error generating wallet: {e}", err=True)

@cli.command("get-address", short_help="Display public address for a wallet file.")
@click.option('--infile', default="wallet.pem", help="Path to the wallet PEM file.", show_default=True, type=click.Path(exists=True, dir_okay=False))
@click.option('--password', help="Password for the wallet file (if encrypted).", default=None)
def get_address_cmd(infile, password):
    """Loads a private key from a PEM file and displays its public address."""
    try:
        priv_key = load_private_key(infile, password)
        pub_key = priv_key.public_key()
        address = public_key_to_address(pub_key)
        click.echo(f"Wallet File: {infile}")
        click.echo(f"Public Address: {address}")
    except Exception as e:
        click.echo(f"Error loading wallet or deriving address: {e}", err=True)

# `load-wallet` is essentially the same as `get-address` for now in terms of output.
# Kept for semantic distinction if more info is displayed later.
@cli.command("load-wallet", short_help="Load a wallet and display its address (alias for get-address).")
@click.option('--infile', default="wallet.pem", help="Path to the wallet PEM file.", show_default=True, type=click.Path(exists=True, dir_okay=False))
@click.option('--password', help="Password for the wallet file (if encrypted).", default=None)
def load_wallet_cmd(infile, password):
    """Loads a private key from a PEM file and displays its public address."""
    try:
        priv_key = load_private_key(infile, password)
        pub_key = priv_key.public_key()
        address = public_key_to_address(pub_key)
        click.echo(f"Wallet File: {infile} loaded successfully.")
        click.echo(f"Public Address: {address}")
    except Exception as e:
        click.echo(f"Error loading wallet: {e}", err=True)


@cli.command("create-transaction", short_help="Create, sign, and send a transaction.")
@click.option('--from-wallet', 'from_wallet_path',required=True, help="Path to the sender's wallet PEM file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--to', 'to_address_hex', required=True, help="Recipient's public address (hex string).")
@click.option('--amount', required=True, type=int, help="Amount to send.")
@click.option('--fee', type=int, default=0, help="Transaction fee (optional).", show_default=True)
@click.option('--password', help="Password for the sender's wallet file (if encrypted).", default=None)
@click.option('--node-url', required=True, help="URL of the Empower1 node (e.g., http://localhost:8080).") # Port 8080 is node's main P2P, debug is 18080. Endpoint will be on main.
def create_transaction_cmd(from_wallet_path, to_address_hex, amount, fee, password, node_url):
    """
    Creates a new transaction, signs it using the sender's wallet,
    and sends it to the specified Empower1 node.
    """
    try:
        # 1. Load sender's private key
        sender_priv_key = load_private_key(from_wallet_path, password)
        sender_pub_key = sender_priv_key.public_key()
        sender_pub_key_bytes = get_public_key_bytes(sender_pub_key)

        # 2. Prepare recipient address bytes
        try:
            # Assuming to_address_hex is the full uncompressed public key hex
            # It should start with '04' and be 130 chars long for P256
            if not to_address_hex.startswith('04') or len(to_address_hex) != 130:
                 click.echo(f"Warning: Recipient address '{to_address_hex}' does not look like a full uncompressed P256 public key hex. Ensure it is correct.", err=True)

            to_address_bytes = bytes.fromhex(to_address_hex)
        except ValueError:
            click.echo(f"Error: Invalid recipient address format. Please provide a hex string.", err=True)
            return

        # 3. Create Transaction object
        tx = Transaction(
            from_address_bytes=sender_pub_key_bytes,
            to_address_bytes=to_address_bytes,
            amount=amount,
            fee=fee,
            public_key_bytes=sender_pub_key_bytes # Pass sender's public key explicitly
        )
        click.echo(f"Transaction created (unsigned): ID={tx.calculate_hash()}, From={public_key_to_address(sender_pub_key)}, To={to_address_hex}, Amount={amount}")

        # 4. Sign the transaction (sign method uses the from_wallet_path again, which is fine)
        tx.sign(from_wallet_path, password)
        click.echo(f"Transaction signed. ID: {tx.id}")

        # 5. Send the transaction
        click.echo(f"Sending transaction to node: {node_url}")
        success, response = send_transaction(node_url, tx)

        if success:
            click.echo(click.style("Transaction submitted successfully!", fg="green"))
            click.echo(f"Node response: {response}")
        else:
            click.echo(click.style(f"Failed to submit transaction.", fg="red"), err=True)
            click.echo(f"Node error response: {response}", err=True)

    except FileNotFoundError:
        click.echo(f"Error: Wallet file not found at {from_wallet_path}", err=True)
    except ValueError as ve: # Catch specific errors like bad password or invalid address format
        click.echo(f"Error: {ve}", err=True)
    except Exception as e:
        click.echo(f"An unexpected error occurred: {e}", err=True)


if __name__ == '__main__':
    # For basic testing of the CLI structure, not commands themselves here.
    # To test commands, run from your terminal:
    # python main.py --help
    # python main.py generate-wallet --outfile mytestwallet.pem
    # python main.py get-address --infile mytestwallet.pem
    cli()
