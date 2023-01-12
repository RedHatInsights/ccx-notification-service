#!/usr/bin/env python3

# Copyright © 2023 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Create plots with all benchmark results."""

import csv
import numpy as np
import matplotlib.pyplot as plt


def plot_results(filename, title, show_graph):
    """Plot and export all results."""
    input_csv = filename + ".csv"

    # Try to open the CSV file specified.
    with open(input_csv) as csv_input:
        # And open this file as CSV
        csv_reader = csv.reader(csv_input)

        rows = 0

        # Read all rows from the provided CSV file
        # and transform benchmark name into readable form
        data = [
            (
                row[0][:-2][len("Benchmark"):].replace("ClusterAs", " "),
                int(row[2]),
            )
            for row in csv_reader
        ]

    # Insert separators
    data.insert(8, ("", 0))
    data.insert(4, (" ", 0))

    print(data)

    # Data to be plotted
    names = [item[0] for item in data]
    speed = [item[1] for item in data]

    # Clear all previous garbage
    plt.figure().clear()

    # Graph
    plt.bar(names, speed)

    # Title of a graph
    plt.title(title)

    # Add a label to x-axis
    plt.xticks(rotation=90)

    # Add a label to y-axis
    plt.ylabel("Speed")

    # Set the plot layout
    plt.tight_layout()

    # And save the plot into raster format and vector format as well
    plt.savefig(filename + ".png")
    plt.savefig(filename + ".svg")

    if show_graph:
        # Try to show the plot on screen
        plt.show()


def main():
    """Perform all plotting."""
    plot_results("normal_100_records", "100 records", False)
    plot_results("normal_1000_records", "1000 records", False)
    plot_results("normal_10000_records", "10000 records", False)
    plot_results("normal_50000_records", "50000 records", False)
    plot_results("normal_100000_records", "100000 records", False)

    plot_results("indexed_100_records", "100 records (indexed)", False)
    plot_results("indexed_1000_records", "1000 records (indexed)", False)
    plot_results("indexed_10000_records", "10000 records (indexed)", False)
    plot_results("indexed_50000_records", "50000 records (indexed)", False)
    plot_results("indexed_100000_records", "100000 records (indexed)", False)


if __name__ == "__main__":
    main()
