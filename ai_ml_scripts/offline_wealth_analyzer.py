import pandas as pd
from sklearn.cluster import KMeans
import numpy as np

# Define the path to the data and the features to use for clustering
DATA_FILE_PATH = "../data/mock_anonymized_blockchain_data.csv" # Path relative to the script location

# Select features for clustering.
# For this example, let's use a subset of the available normalized features.
# More sophisticated feature selection/engineering would be done in a real scenario.
FEATURE_COLUMNS = [
    'tx_frequency_norm',
    'tx_volume_sum_norm',
    'counterparty_count_norm',
    'net_value_flow_norm',
    'contract_interaction_freq_norm',
    'gas_used_sum_norm'
]

# Define the number of clusters (K)
# This could be determined using methods like the Elbow method or Silhouette analysis.
# For this conceptual example, we'll set it to a common small number, e.g., 3 or 4.
N_CLUSTERS = 3 # Example: Low, Medium, High activity/wealth proxy tiers

def load_and_prepare_data(file_path, feature_columns):
    """Loads data from CSV and selects features for clustering."""
    try:
        df = pd.read_csv(file_path)
    except FileNotFoundError:
        print(f"Error: Data file not found at {file_path}")
        print("Please ensure the mock data CSV is in the correct directory (e.g., ../data/ from script location).")
        return None, None

    # Ensure all selected feature columns are present in the DataFrame
    missing_cols = [col for col in feature_columns if col not in df.columns]
    if missing_cols:
        print(f"Error: The following feature columns are missing from the CSV file: {missing_cols}")
        return None, None

    # Handle potential NaN or infinity values if any (though mock data should be clean)
    df = df.replace([np.inf, -np.inf], np.nan)
    if df[feature_columns].isnull().values.any():
        print("Warning: NaN values found in feature columns. Filling with 0 for this example.")
        df[feature_columns] = df[feature_columns].fillna(0)

    X = df[feature_columns]
    ids = df['anonymized_id']
    return X, ids

def perform_kmeans_clustering(X, n_clusters):
    """Performs K-Means clustering on the given data X."""
    if X is None or X.empty:
        print("Error: Feature set X is empty or None. Cannot perform clustering.")
        return None

    kmeans = KMeans(n_clusters=n_clusters, random_state=42, n_init='auto') # n_init='auto' to suppress warning
    try:
        cluster_labels = kmeans.fit_predict(X)
    except Exception as e:
        print(f"Error during K-Means fitting: {e}")
        return None

    return cluster_labels, kmeans.cluster_centers_

def main():
    print("Starting Offline Wealth Analyzer (Conceptual K-Means Clustering)...")

    X, ids = load_and_prepare_data(DATA_FILE_PATH, FEATURE_COLUMNS)

    if X is None:
        print("Exiting due to data loading issues.")
        return

    print(f"\nPerforming K-Means clustering with K={N_CLUSTERS} on {len(X)} entities using features: {FEATURE_COLUMNS}")

    cluster_labels, cluster_centers = perform_kmeans_clustering(X, N_CLUSTERS)

    if cluster_labels is None:
        print("Clustering failed. Exiting.")
        return

    print("\n--- Clustering Results ---")
    results_df = pd.DataFrame({'anonymized_id': ids, 'cluster_id': cluster_labels})
    results_df = results_df.sort_values(by='cluster_id')

    for i in range(N_CLUSTERS):
        cluster_data = results_df[results_df['cluster_id'] == i]
        print(f"\nCluster {i} (Size: {len(cluster_data)}):")
        print(cluster_data['anonymized_id'].tolist())

    if cluster_centers is not None:
        print("\n--- Cluster Centroids (Mean Feature Values per Cluster) ---")
        centroids_df = pd.DataFrame(cluster_centers, columns=FEATURE_COLUMNS)
        print(centroids_df)
        print("\nInterpretation of Centroids:")
        print("  - Higher values in features like 'tx_frequency_norm', 'tx_volume_sum_norm' might indicate 'higher activity/wealth' clusters.")
        print("  - Lower values might indicate 'lower activity/wealth' clusters.")
        print("  - 'net_value_flow_norm' being positive might indicate accumulation, negative might indicate spending/distribution.")

    print("\n--- Important Considerations ---")
    print("1. This is a conceptual POC using MOCK, ANONYMIZED, and NORMALIZED data.")
    print("2. The number of clusters (K) was preset. Optimal K needs proper evaluation (e.g., Elbow method).")
    print("3. Feature selection and engineering are crucial for meaningful results in a real scenario.")
    print("4. Interpretation of clusters is subjective and depends on the chosen features and business logic.")
    print("5. Ethical implications and privacy preservation are paramount if ever applied to real data.")

if __name__ == "__main__":
    main()
