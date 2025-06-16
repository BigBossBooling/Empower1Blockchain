import click
import os
from wallet import (
    generate_key_pair,
    save_private_key,
    load_private_key,
    public_key_to_address,
    get_public_key_bytes
)
from transaction import Transaction, TX_STANDARD, TX_CONTRACT_DEPLOY, TX_CONTRACT_CALL, SignerInfo
from client import send_transaction
from multisig import create_multisig_config, load_multisig_config
from did_utils import generate_did_key_from_public_key_hex
import json
import requests # For did get-info using debug endpoint

CONTEXT_SETTINGS = dict(help_option_names=['-h', '--help'])

@click.group(context_settings=CONTEXT_SETTINGS)
def cli():
    """Empower1 CLI Wallet"""
    pass

@cli.command("generate-wallet", short_help="Generate a new ECDSA keypair and save it.")
@click.option('--outfile', default="wallet.pem", help="Output file for the wallet.", show_default=True)
@click.option('--password', help="Password to encrypt the wallet file (optional).", default=None)
def generate_wallet_cmd(outfile, password):
    try:
        priv_key, pub_key = generate_key_pair()
        save_private_key(priv_key, outfile, password)
        address = public_key_to_address(pub_key)
        click.echo(f"New wallet generated and saved to: {outfile}")
        click.echo(f"Public Address (Hex of Uncompressed PubKey): {address}")
    except Exception as e:
        click.echo(f"Error generating wallet: {e}", err=True)

@cli.command("get-address", short_help="Display public address for a wallet file.")
@click.option('--infile', default="wallet.pem", help="Path to the wallet PEM file.", show_default=True, type=click.Path(exists=True, dir_okay=False))
@click.option('--password', help="Password for the wallet file (if encrypted).", default=None)
def get_address_cmd(infile, password):
    try:
        priv_key = load_private_key(infile, password)
        pub_key = priv_key.public_key()
        address = public_key_to_address(pub_key)
        click.echo(f"Wallet File: {infile}")
        click.echo(f"Public Address (Hex of Uncompressed PubKey): {address}")
    except Exception as e:
        click.echo(f"Error loading wallet or deriving address: {e}", err=True)

@cli.command("create-transaction", short_help="Create, sign, and send a single-signer transaction.")
@click.option('--from-wallet', 'from_wallet_path',required=True, help="Path to the sender's wallet PEM file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--to', 'to_address_hex', required=True, help="Recipient's public address (hex string).")
@click.option('--amount', required=True, type=int, help="Amount to send.")
@click.option('--fee', type=int, default=0, help="Transaction fee (optional).", show_default=True)
@click.option('--password', help="Password for the sender's wallet file (if encrypted).", default=None)
@click.option('--node-url', required=True, help="URL of the Empower1 node.")
def create_transaction_cmd(from_wallet_path, to_address_hex, amount, fee, password, node_url):
    try:
        sender_priv_key = load_private_key(from_wallet_path, password)
        sender_pub_key = sender_priv_key.public_key()
        sender_pub_key_hex = get_public_key_bytes(sender_pub_key).hex()

        tx = Transaction(
            from_address_hex=sender_pub_key_hex,
            to_address_hex=to_address_hex,
            amount=amount,
            fee=fee,
            public_key_hex=sender_pub_key_hex,
            tx_type=TX_STANDARD
        )
        click.echo(f"Transaction created (unsigned): From={tx.from_address_hex}, To={tx.to_address_hex}, Amount={tx.amount}")

        tx.sign_single(from_wallet_path, password)
        click.echo(f"Transaction signed. ID: {tx.id_hex}")

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
    except ValueError as ve:
        click.echo(f"Error: {ve}", err=True)
    except Exception as e:
        click.echo(f"An unexpected error occurred: {e}", err=True)

@cli.group("multisig", short_help="Manage and use multi-signature wallets/transactions.")
def multisig():
    pass

@multisig.command("create-config", short_help="Create a multi-signature configuration file.")
@click.option('-m', '--m-required', type=int, required=True, help="Number of required signatures (M).")
@click.option('--signer-wallet', 'signer_wallets', type=click.Path(exists=True, dir_okay=False), required=True, multiple=True, help="Path to a signer's PEM wallet file (repeat for N signers).")
@click.option('--signer-password', 'signer_passwords', help="Password for the corresponding signer's wallet (repeat in same order). Use empty string or omit for no password for a specific wallet if possible with multiple.", multiple=True, default=[None])
@click.option('--outfile', default="multisig_config.json", help="Output file for the multi-sig configuration.", show_default=True)
def multisig_create_config_cmd(m_required, signer_wallets, signer_passwords, outfile):
    if len(signer_wallets) == 0:
        click.echo("Error: At least one signer wallet must be provided for N.", err=True); return
    final_passwords = list(signer_passwords)
    if len(final_passwords) == 1 and final_passwords[0] is None and len(signer_wallets) > 1:
        final_passwords = [None] * len(signer_wallets)
    elif len(final_passwords) != len(signer_wallets):
        click.echo(f"Error: Mismatch between number of signer wallets ({len(signer_wallets)}) and passwords ({len(final_passwords)}).", err=True); return
    try:
        create_multisig_config(outfile, m_required, list(signer_wallets), final_passwords)
    except Exception as e:
        click.echo(f"Error creating multi-sig config: {e}", err=True)

@multisig.command("initiate-tx", short_help="Initiate a new multi-signature transaction.")
@click.option('--config', 'config_file', required=True, help="Path to multi-sig config JSON file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--to', 'to_address_hex', required=True, help="Recipient's public address (hex string).")
@click.option('--amount', required=True, type=int, help="Amount to send.")
@click.option('--fee', type=int, default=0, help="Transaction fee.", show_default=True)
@click.option('--tx-type', 'tx_type_str', type=click.Choice([TX_STANDARD]), default=TX_STANDARD, show_default=True, help="Type of transaction (currently only standard).")
@click.option('--outfile', default="pending_multisig_tx.json", help="Output file for the initiated transaction.", show_default=True)
def multisig_initiate_tx_cmd(config_file, to_address_hex, amount, fee, tx_type_str, outfile):
    try:
        config = load_multisig_config(config_file)
        tx = Transaction(
            from_address_hex=config["multisig_address_hex"], to_address_hex=to_address_hex, amount=amount, fee=fee,
            tx_type=tx_type_str, required_signatures=config["m_required"],
            authorized_public_keys_hex=config["authorized_public_keys_hex"], signers=[] )
        tx.id_hex = tx.calculate_hash()
        with open(outfile, 'w') as f: json.dump(tx.to_dict_for_file(), f, indent=4)
        click.echo(f"Multi-sig transaction initiated to: {outfile}\n  ID (to be signed): {tx.id_hex}\n  From (Multi-sig Addr): {tx.from_address_hex}\n  Requires {tx.required_signatures} of {len(tx.authorized_public_keys_hex)} sigs.")
    except Exception as e: click.echo(f"Error initiating multi-sig transaction: {e}", err=True)

@multisig.command("sign-tx", short_help="Add a signature to a multi-signature transaction.")
@click.option('--pending-tx', 'pending_tx_file', required=True, help="Path to pending multi-sig JSON transaction file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--wallet', 'signer_wallet_pem', required=True, help="Path to the signer's wallet PEM file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--password', help="Password for the signer's wallet (if encrypted).", default=None)
@click.option('--outfile', 'signed_tx_file', help="Output file for the updated transaction (can be same as pending-tx).", default=None)
def multisig_sign_tx_cmd(pending_tx_file, signer_wallet_pem, password, signed_tx_file):
    try:
        with open(pending_tx_file, 'r') as f: tx_data = json.load(f)
        tx = Transaction.from_dict_for_file(tx_data)
        original_signer_count = len(tx.signers)
        tx.add_signature(signer_wallet_pem, password)
        if len(tx.signers) > original_signer_count: click.echo(click.style("Signature added successfully.", fg="green"))
        else: click.echo(click.style("Signature may have already been present.", fg="yellow"))
        output_file = signed_tx_file if signed_tx_file else pending_tx_file
        with open(output_file, 'w') as f: json.dump(tx.to_dict_for_file(), f, indent=4)
        click.echo(f"Updated transaction saved to: {output_file}\n  ID: {tx.id_hex}\n  Signers: {len(tx.signers)} / {tx.required_signatures} required.")
    except Exception as e: click.echo(f"Error signing multi-sig transaction: {e}", err=True)

@multisig.command("broadcast-tx", short_help="Broadcast a signed multi-sig transaction.")
@click.option('--signed-tx', 'signed_tx_file', required=True, help="Path to signed multi-sig JSON transaction file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--node-url', required=True, help="URL of the Empower1 node.")
def multisig_broadcast_tx_cmd(signed_tx_file, node_url):
    try:
        with open(signed_tx_file, 'r') as f: tx_data = json.load(f)
        tx = Transaction.from_dict_for_file(tx_data)
        if len(tx.signers) < tx.required_signatures:
            click.echo(f"Warning: Tx has {len(tx.signers)}/{tx.required_signatures} sigs.", err=True)
            if not click.confirm("Broadcast anyway?"): return
        click.echo(f"Broadcasting multi-sig transaction ID: {tx.id_hex} to node: {node_url}")
        success, response = send_transaction(node_url, tx)
        if success: click.echo(click.style(f"Multi-sig transaction submitted: {response.get('tx_id', 'OK')}", fg="green")); click.echo(f"Node response: {response}")
        else: click.echo(click.style(f"Failed to submit multi-sig tx: {response.get('error', 'Unknown')}", fg="red"), err=True)
    except Exception as e: click.echo(f"Error broadcasting multi-sig transaction: {e}", err=True)

@cli.group("did", short_help="Manage Decentralized Identifiers (DIDs).")
def did():
    pass

@did.command("generate", short_help="Generate a did:key from a wallet.")
@click.option('--wallet', 'wallet_path',required=True, help="Path to the wallet PEM file.", type=click.Path(exists=True, dir_okay=False))
@click.option('--password', help="Password for the wallet file (if encrypted).", default=None)
def did_generate_cmd(wallet_path, password):
    try:
        priv_key = load_private_key(wallet_path, password)
        pub_key = priv_key.public_key()
        pub_key_hex = get_public_key_bytes(pub_key).hex()
        did_key_string = generate_did_key_from_public_key_hex(pub_key_hex)
        click.echo(f"Wallet File: {wallet_path}")
        click.echo(f"Public Key Hex: {pub_key_hex}")
        click.echo(click.style(f"DID Key: {did_key_string}", fg="green"))
    except Exception as e:
        click.echo(f"Error generating DID: {e}", err=True)

@did.command("register-document", short_help="Register/update a DID document reference on-chain.")
@click.option('--wallet', 'wallet_path', required=True, help="Wallet PEM file of the DID owner (for signing).", type=click.Path(exists=True, dir_okay=False))
@click.option('--password', help="Password for the wallet file.", default=None)
@click.option('--did', 'did_string', required=True, help="The did:key string to register/update (must match wallet).")
@click.option('--doc-hash', required=True, help="Cryptographic hash of the DID document.")
@click.option('--doc-uri', required=True, help="URI where the DID document can be resolved.")
@click.option('--contract-address', 'contract_address_hex', required=True, help="Hex address of the DIDRegistry smart contract.")
@click.option('--node-url', required=True, help="URL of the Empower1 node.")
@click.option('--fee', type=int, default=10, help="Transaction fee.", show_default=True)
def did_register_document_cmd(wallet_path, password, did_string, doc_hash, doc_uri, contract_address_hex, node_url, fee):
    try:
        signer_priv_key = load_private_key(wallet_path, password)
        signer_pub_key = signer_priv_key.public_key()
        signer_pub_key_hex = get_public_key_bytes(signer_pub_key).hex()

        expected_did_from_wallet = generate_did_key_from_public_key_hex(signer_pub_key_hex)
        if did_string != expected_did_from_wallet:
            click.echo(f"Error: Provided DID '{did_string}' does not match wallet's DID '{expected_did_from_wallet}'.", err=True); return

        click.echo(f"Preparing to register DID: {did_string}\n  Doc Hash: {doc_hash}\n  Doc URI: {doc_uri}\n  Contract: {contract_address_hex}")

        contract_args_list = [did_string, doc_hash, doc_uri] # Order for registerDIDDocument(did, hash, uri)
        arguments_json_string = json.dumps(contract_args_list)
        arguments_bytes = arguments_json_string.encode('utf-8')

        tx = Transaction(
            from_address_hex=signer_pub_key_hex,
            tx_type=TX_CONTRACT_CALL,
            target_contract_address_hex=contract_address_hex,
            function_name="registerDIDDocument",
            arguments_bytes=arguments_bytes,
            fee=fee,
            public_key_hex=signer_pub_key_hex
        )
        tx.sign_single(wallet_path, password)
        click.echo(f"DID registration transaction signed. ID: {tx.id_hex}")
        success, response = send_transaction(node_url, tx)
        if success: click.echo(click.style(f"DID registration tx submitted: {response.get('tx_id', 'OK')}", fg="green")); click.echo(f"Node response: {response}")
        else: click.echo(click.style(f"Failed to submit DID registration tx: {response.get('error', 'Unknown')}", fg="red"), err=True)
    except Exception as e: click.echo(f"An unexpected error occurred: {e}", err=True)

@did.command("get-info", short_help="Get document info for a DID from the registry.")
@click.option('--did', 'did_string', required=True, help="The did:key string to query.")
@click.option('--contract-address', 'contract_address_hex', required=True, help="Hex address of the DIDRegistry smart contract.")
@click.option('--node-url', required=True, help="URL of the Empower1 node.")
def did_get_info_cmd(did_string, contract_address_hex, node_url):
    try:
        click.echo(f"Querying DID: {did_string} via contract: {contract_address_hex}")
        contract_args_list = [did_string]
        arguments_json_string = json.dumps(contract_args_list)

        call_payload = {
            "contract_address": contract_address_hex,
            "function_name": "getDIDDocumentInfo",
            "arguments_json": arguments_json_string,
            "gas_limit": 1000000
        }
        if not node_url.startswith("http"): node_url = f"http://{node_url}"
        debug_call_endpoint = f"{node_url}/debug/call-contract"
        headers = {'Content-Type': 'application/json'}
        response = requests.post(debug_call_endpoint, data=json.dumps(call_payload), headers=headers, timeout=10)

        if response.status_code == 200:
            response_data = response.json()
            click.echo(click.style("Call to getDIDDocumentInfo successful.", fg="green"))
            click.echo(f"  Gas Consumed: {response_data.get('gas_consumed')}")
            click.echo(f"  Return Value (JSON string or null): {response_data.get('contract_result')}")
        else:
            click.echo(click.style(f"Failed to call getDIDDocumentInfo. Status: {response.status_code}", fg="red"), err=True)
            try: click.echo(f"Error: {response.json()}", err=True)
            except: click.echo(f"Error: {response.text}", err=True)
    except Exception as e: click.echo(f"An unexpected error occurred: {e}", err=True)

if __name__ == '__main__':
    cli()
