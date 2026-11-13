#!/usr/bin/gnuplot

# Set terminal and output file
set terminal png size 600,400
#set grid

set title 'Cosine Wave Input Data'
set xlabel 'Entries'
set ylabel 'Size'
set output 'cosine_wave_plot_points_input.png'
plot 'input.dat' using 1:2 with linespoints title 'Cosine Wave' linewidth 2 pointtype 7 pointsize 0.5


set xlabel 'Seconds'
set ylabel 'Write Op/s'
set output 'cosine_wave_static_plot_points.png'

stats 'adaptive_rl_output.dat' using 1 nooutput
start_adaptive_rl_time = STATS_min

stats 'static_rl_output.dat' using 1 nooutput
start_rl_time = STATS_min

set xrange [0:2010]
set yrange [0:5200]
plot 'static_rl_output.dat' using ($1 - start_rl_time):2 \
  title 'Static Rate Limiter' pointtype 1 pointsize 0.6

set xlabel 'Seconds'
set ylabel 'Write Op/s'
set output 'cosine_wave_adaptive_plot_points.png'
plot 'adaptive_rl_output.dat' using ($1 - start_adaptive_rl_time):2 \
  title 'Adaptive Rate Limiter' pointtype 1 pointsize 0.6
