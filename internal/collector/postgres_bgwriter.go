package collector

import (
	"github.com/barcodepro/pgscv/internal/log"
	"github.com/barcodepro/pgscv/internal/model"
	"github.com/barcodepro/pgscv/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	postgresBgwriterQuery = `SELECT
  checkpoints_timed, checkpoints_req,
  checkpoint_write_time, checkpoint_sync_time,
  buffers_checkpoint, buffers_clean, maxwritten_clean,
  buffers_backend, buffers_backend_fsync, buffers_alloc,
  coalesce(extract('epoch' from age(now(), stats_reset)), 0) as stats_age_seconds
FROM pg_stat_bgwriter`
)

type postgresBgwriterCollector struct {
	descs map[string]typedDesc
}

// NewPostgresBgwriterCollector returns a new Collector exposing postgres bgwriter and checkpointer stats.
// For details see https://www.postgresql.org/docs/current/monitoring-stats.html#PG-STAT-BGWRITER-VIEW
func NewPostgresBgwriterCollector(constLabels prometheus.Labels) (Collector, error) {
	labels := []string{"type"}

	return &postgresBgwriterCollector{
		descs: map[string]typedDesc{
			"checkpoints": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "ckpt_total"),
					"Total number of checkpoints that have been performed of each type.",
					[]string{"ckpt"}, constLabels,
				), valueType: prometheus.CounterValue,
			},
			"checkpoint_time": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "ckpt_time_seconds_total"),
					"Total amount of time that has been spent writing or syncing data during checkpoint, in seconds.",
					[]string{"op"}, constLabels,
				), valueType: prometheus.CounterValue, factor: .001,
			},
			"buffers_written": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "buffers_written_total"),
					"Total number of buffers written.",
					labels, constLabels,
				), valueType: prometheus.CounterValue,
			},
			"maxwritten_clean": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "bgwr_maxwritten_clean_total"),
					"Total number of times the background writer stopped a cleaning scan because it had written too many buffers.",
					nil, constLabels,
				), valueType: prometheus.CounterValue,
			},
			"buffers_backend_fsync": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "backend_fsync_total"),
					"Total number of times a backend had to execute its own fsync call.",
					nil, constLabels,
				), valueType: prometheus.CounterValue,
			},
			"buffers_alloc": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "backend_buffers_allocated_total"),
					"Total number of buffers allocated.",
					nil, constLabels,
				), valueType: prometheus.CounterValue,
			},
			"stats_age_seconds": {
				desc: prometheus.NewDesc(
					prometheus.BuildFQName("postgres", "bgwriter", "stats_age_seconds"),
					"The age of the activity statistics, in seconds.",
					nil, constLabels,
				), valueType: prometheus.CounterValue,
			},
		},
	}, nil
}

// Update method collects statistics, parse it and produces metrics that are sent to Prometheus.
func (c *postgresBgwriterCollector) Update(config Config, ch chan<- prometheus.Metric) error {
	conn, err := store.New(config.ConnString)
	if err != nil {
		return err
	}
	defer conn.Close()

	res, err := conn.Query(postgresBgwriterQuery)
	if err != nil {
		return err
	}

	stats := parsePostgresBgwriterStats(res)

	for name, desc := range c.descs {
		switch name {
		case "checkpoints":
			ch <- desc.mustNewConstMetric(stats.ckptTimed, "timed")
			ch <- desc.mustNewConstMetric(stats.ckptReq, "req")
		case "checkpoint_time":
			ch <- desc.mustNewConstMetric(stats.ckptWriteTime, "write")
			ch <- desc.mustNewConstMetric(stats.ckptSyncTime, "sync")
		case "maxwritten_clean":
			ch <- desc.mustNewConstMetric(stats.bgwrMaxWritten)
		case "buffers_written":
			ch <- desc.mustNewConstMetric(stats.ckptBuffers, "checkpointer")
			ch <- desc.mustNewConstMetric(stats.bgwrBuffers, "bgwriter")
			ch <- desc.mustNewConstMetric(stats.backendBuffers, "backend")
		case "buffers_backend_fsync":
			ch <- desc.mustNewConstMetric(stats.backendFsync)
		case "buffers_alloc":
			ch <- desc.mustNewConstMetric(stats.backendAllocated)
		case "stats_age_seconds":
			ch <- desc.mustNewConstMetric(stats.statsAgeSeconds)
		default:
			log.Debugf("unknown desc name: %s, skip", name)
			continue
		}
	}

	return nil
}

// postgresBgwriterStat describes stats related to Postgres background writes.
type postgresBgwriterStat struct {
	ckptTimed        float64
	ckptReq          float64
	ckptWriteTime    float64
	ckptSyncTime     float64
	ckptBuffers      float64
	bgwrBuffers      float64
	bgwrMaxWritten   float64
	backendBuffers   float64
	backendFsync     float64
	backendAllocated float64
	statsAgeSeconds  float64
}

// parsePostgresBgwriterStats parses PGResult and returns struct with data values
func parsePostgresBgwriterStats(r *model.PGResult) postgresBgwriterStat {
	var stats postgresBgwriterStat

	for _, row := range r.Rows {
		for i, colname := range r.Colnames {
			// Skip empty (NULL) values.
			if row[i].String == "" {
				log.Debug("got empty (NULL) value, skip")
				continue
			}

			// Get data value and convert it to float64 used by Prometheus.
			v, err := strconv.ParseFloat(row[i].String, 64)
			if err != nil {
				log.Errorf("skip collecting metric: %s", err)
				continue
			}

			// Update stats struct
			switch string(colname.Name) {
			case "checkpoints_timed":
				stats.ckptTimed = v
			case "checkpoints_req":
				stats.ckptReq = v
			case "checkpoint_write_time":
				stats.ckptWriteTime = v
			case "checkpoint_sync_time":
				stats.ckptSyncTime = v
			case "buffers_checkpoint":
				stats.ckptBuffers = v
			case "buffers_clean":
				stats.bgwrBuffers = v
			case "maxwritten_clean":
				stats.bgwrMaxWritten = v
			case "buffers_backend":
				stats.backendBuffers = v
			case "buffers_backend_fsync":
				stats.backendFsync = v
			case "buffers_alloc":
				stats.backendAllocated = v
			case "stats_age_seconds":
				stats.statsAgeSeconds = v
			default:
				log.Debugf("unsupported pg_stat_bgwriter stat column: %s, skip", string(colname.Name))
				continue
			}
		}
	}

	return stats
}