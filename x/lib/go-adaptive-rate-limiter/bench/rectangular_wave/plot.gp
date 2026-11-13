#!/usr/bin/gnuplot

# Set terminal and output file
set terminal png size 800,600
set grid

set title 'Rectangular Wave Input Data'
set xlabel 'Entries'
set ylabel 'Size'
set output 'rectangular_wave_plot_points_input.png'
plot 'input.dat' using 1:2 with linespoints title 'Rectangular Wave' linewidth 2 pointtype 7 pointsize 0.5


set xlabel 'Seconds'
set ylabel 'Write Op/s'
set output 'rectangular_wave_plot_points.png'

stats 'adaptive_rl_output.dat' using 1 nooutput
start_adaptive_rl_time = STATS_min

stats 'static_rl_output.dat' using 1 nooutput
start_rl_time = STATS_min

set xrange [0:550]
set yrange [0:6050]
plot \
  'static_rl_output.dat' using ($1 - start_rl_time):2 with linespoints title 'Static Rate Limiter' linewidth 2 pointtype 7 pointsize 0.5, \
  'adaptive_rl_output.dat' using ($1 - start_adaptive_rl_time):2 with linespoints title 'Adaptive Rate Limiter' linewidth 2 pointtype 7 pointsize 0.5
